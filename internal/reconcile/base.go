// Package reconcile provides a generic base reconciler that owns the parts of a
// controller-runtime reconcile loop that are identical across every HCloud* kind:
// fetch, finalizer management, deletion, status persistence with retry-on-conflict,
// the "Synced" condition, Kubernetes Events, and reconcile metrics.
//
// Each controller supplies only its domain logic by implementing Resource[T].
// This keeps observability consistent and prevents per-controller drift.
package reconcile

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/armanfeyzi/hcloud-operator/internal/metrics"
)

// Condition types owned (Synced) or shared (Ready) across controllers.
const (
	// ConditionSynced reports whether the last reconcile loop completed without
	// error (control-plane health). It is owned entirely by the base.
	ConditionSynced = "Synced"

	// ConditionReady reports whether the external Hetzner resource is provisioned
	// and usable (data-plane). It is owned by domain logic; the base only forces it
	// to False on a reconcile error so a failing resource never appears Ready.
	ConditionReady = "Ready"

	reasonReconcileError   = "ReconcileError"
	reasonReconcileSuccess = "ReconcileSuccess"

	// metric result label values (match internal/metrics conventions).
	metricResultSuccess = "success"
	metricResultError   = "error"
)

// Event reasons emitted by the base.
const (
	eventDeleted        = "Deleted"
	eventDeleteFailed   = "DeleteFailed"
	eventReconcileError = reasonReconcileError
)

// Managed is implemented by every HCloud* root type. It is a client.Object that
// also exposes a pointer to its status conditions so the base can set "Synced".
type Managed interface {
	client.Object
	GetConditions() *[]metav1.Condition
}

// Resource is the per-kind domain contract supplied by each controller.
type Resource[T Managed] interface {
	// NewObject returns a fresh, empty instance of the managed type (e.g. &HCloudServer{}).
	NewObject() T
	// FinalizerName is the finalizer this controller manages.
	FinalizerName() string
	// Kind is a short, stable label used for events and metrics (e.g. "HCloudServer").
	Kind() string
	// Reconcile converges external state for an existing, non-deleted object.
	// It may set the Ready condition and persist domain status fields itself.
	Reconcile(ctx context.Context, obj T) (ctrl.Result, error)
	// Delete performs external cleanup before the finalizer is removed.
	Delete(ctx context.Context, obj T) error
}

// Cross-cutting RBAC introduced by the base reconciler:
//   - leases: required by controller-runtime leader election (now on by default).
//   - events: required so the base (and domain logic) can emit Kubernetes Events.
//
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// BaseReconciler drives the shared reconcile skeleton for a single kind.
type BaseReconciler[T Managed] struct {
	client.Client
	Recorder record.EventRecorder
	Resource Resource[T]
}

// Reconcile implements controller-runtime's reconcile.Reconciler for any Managed kind.
func (b *BaseReconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kind := b.Resource.Kind()
	start := time.Now()

	obj := b.Resource.NewObject()
	if err := b.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// Object was deleted before reconcile ran — nothing to do.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get %s: %w", kind, err)
	}

	// ── Deletion ────────────────────────────────────────────────────────────
	if !obj.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(obj, b.Resource.FinalizerName()) {
			if err := b.Resource.Delete(ctx, obj); err != nil {
				b.event(obj, corev1.EventTypeWarning, eventDeleteFailed, err.Error())
				return ctrl.Result{}, fmt.Errorf("delete %s external resource: %w", kind, err)
			}
			controllerutil.RemoveFinalizer(obj, b.Resource.FinalizerName())
			if err := b.Update(ctx, obj); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove %s finalizer: %w", kind, err)
			}
			b.event(obj, corev1.EventTypeNormal, eventDeleted, "External resource deleted and finalizer removed")
		}
		return ctrl.Result{}, nil
	}

	// ── Ensure finalizer ─────────────────────────────────────────────────────
	if !controllerutil.ContainsFinalizer(obj, b.Resource.FinalizerName()) {
		controllerutil.AddFinalizer(obj, b.Resource.FinalizerName())
		if err := b.Update(ctx, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("add %s finalizer: %w", kind, err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// ── Delegate to domain logic ─────────────────────────────────────────────
	result, err := b.Resource.Reconcile(ctx, obj)
	if err != nil {
		b.setCondition(obj, ConditionSynced, metav1.ConditionFalse, reasonReconcileError, err.Error())
		b.setCondition(obj, ConditionReady, metav1.ConditionFalse, reasonReconcileError, err.Error())
		b.event(obj, corev1.EventTypeWarning, eventReconcileError, err.Error())
		metrics.RecordReconcile(kind, metricResultError, time.Since(start))
		// Best-effort status persistence; return the original error to requeue.
		_ = b.updateStatusWithRetry(ctx, obj)
		return ctrl.Result{}, err
	}

	b.setCondition(obj, ConditionSynced, metav1.ConditionTrue, reasonReconcileSuccess, "Reconcile loop completed without error")
	metrics.RecordReconcile(kind, metricResultSuccess, time.Since(start))
	if err := b.updateStatusWithRetry(ctx, obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("persist %s status: %w", kind, err)
	}
	return result, nil
}

// setCondition upserts a condition on the managed object's status.
func (b *BaseReconciler[T]) setCondition(obj T, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

// event records a Kubernetes Event when a recorder is configured.
func (b *BaseReconciler[T]) event(obj T, eventType, reason, message string) {
	if b.Recorder != nil {
		b.Recorder.Event(obj, eventType, reason, message)
	}
}

// updateStatusWithRetry persists the object's status subresource, retrying on
// optimistic-concurrency conflicts by refreshing only the resourceVersion while
// keeping the desired in-memory status. This is the generic promotion of the
// per-controller updateServerStatusWithRetry helper.
func (b *BaseReconciler[T]) updateStatusWithRetry(ctx context.Context, obj T) error {
	key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := b.Status().Update(ctx, obj)
		if apierrors.IsConflict(err) {
			latest := b.Resource.NewObject()
			if getErr := b.Get(ctx, key, latest); getErr != nil {
				return getErr
			}
			obj.SetResourceVersion(latest.GetResourceVersion())
		}
		return err
	})
}
