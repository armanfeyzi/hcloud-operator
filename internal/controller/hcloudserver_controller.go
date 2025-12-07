package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/afeyzirealyticsio/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/afeyzirealyticsio/hcloud-operator/internal/hcloud"
)

const (
	// hcloudServerFinalizer is the finalizer added to HCloudServer resources.
	// The controller will not allow deletion until Hetzner-side cleanup is complete.
	hcloudServerFinalizer = "infra.hkc.io/finalizer"

	// requeueDelay is the default requeue interval for non-error reconciliations.
	requeueDelay = 30 * time.Second

	// conditionTypeReady is the condition type used to indicate resource readiness.
	conditionTypeReady = "Ready"
)

// HCloudServerReconciler reconciles HCloudServer objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers/finalizers,verbs=update
type HCloudServerReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient *hcloudclient.Client
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *HCloudServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudServer{}).
		Complete(r)
}

// Reconcile is the core reconciliation loop for HCloudServer resources.
// It is called every time a HCloudServer is created, updated, or deleted.
func (r *HCloudServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// ── 1. Fetch the HCloudServer resource ──────────────────────────────────
	server := &infrav1alpha1.HCloudServer{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		if apierrors.IsNotFound(err) {
			// Object deleted before reconcile ran — nothing to do.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get HCloudServer: %w", err)
	}

	// ── 2. Handle deletion ───────────────────────────────────────────────────
	if !server.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(server, hcloudServerFinalizer) {
			log.Info("Handling deletion", "serverID", server.Status.ServerID)
			if err := r.deleteHCloudServer(ctx, server); err != nil {
				return ctrl.Result{}, fmt.Errorf("delete HCloud server: %w", err)
			}
			controllerutil.RemoveFinalizer(server, hcloudServerFinalizer)
			if err := r.Update(ctx, server); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// ── 3. Ensure finalizer is present ───────────────────────────────────────
	if !controllerutil.ContainsFinalizer(server, hcloudServerFinalizer) {
		controllerutil.AddFinalizer(server, hcloudServerFinalizer)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// ── 4. Reconcile Hetzner server ──────────────────────────────────────────
	if err := r.reconcileHCloudServer(ctx, server); err != nil {
		r.setCondition(server, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.Status().Update(ctx, server)
		return ctrl.Result{}, err
	}

	// ── 5. Requeue periodically to drift-correct ─────────────────────────────
	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

// reconcileHCloudServer ensures the Hetzner Cloud server matches the desired spec.
func (r *HCloudServerReconciler) reconcileHCloudServer(ctx context.Context, obj *infrav1alpha1.HCloudServer) error {
	log := log.FromContext(ctx)

	// Try to find existing server by stored ID first, then by name.
	var hcServer interface{ GetID() int64 }

	if obj.Status.ServerID != 0 {
		existing, err := r.HCloudClient.GetServer(ctx, obj.Status.ServerID)
		if err != nil {
			return fmt.Errorf("fetch server by ID: %w", err)
		}
		if existing != nil {
			// Server exists — sync status.
			log.Info("Server exists, syncing status", "serverID", existing.ID)
			obj.Status.State = string(existing.Status)
			if existing.PublicNet.IPv4.IP != nil {
				obj.Status.PublicIPv4 = existing.PublicNet.IPv4.IP.String()
			}
			if existing.PublicNet.IPv6.IP != nil {
				obj.Status.PublicIPv6 = existing.PublicNet.IPv6.Network.String()
			}
			r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, "ServerRunning", "Hetzner server is running")
			return r.Status().Update(ctx, obj)
		}
		// Server was deleted externally — recreate.
		log.Info("Server not found by ID, recreating", "serverID", obj.Status.ServerID)
	}

	_ = hcServer // suppress unused warning until full implementation

	// Create the server.
	log.Info("Creating new Hetzner server", "name", obj.Name, "serverType", obj.Spec.ServerType)
	created, err := r.HCloudClient.CreateServer(ctx, hcloudclient.ServerCreateOpts{
		Name:       obj.Name,
		ServerType: obj.Spec.ServerType,
		Image:      obj.Spec.Image,
		Location:   obj.Spec.Location,
		Labels:     obj.Spec.Labels,
		SSHKeys:    obj.Spec.SSHKeys,
		UserData:   obj.Spec.UserData,
	})
	if err != nil {
		return fmt.Errorf("create Hetzner server: %w", err)
	}

	obj.Status.ServerID = created.ID
	obj.Status.State = string(created.Status)
	if created.PublicNet.IPv4.IP != nil {
		obj.Status.PublicIPv4 = created.PublicNet.IPv4.IP.String()
	}
	r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, "ServerCreated", "Hetzner server created successfully")

	return r.Status().Update(ctx, obj)
}

// deleteHCloudServer removes the Hetzner Cloud server for this resource.
func (r *HCloudServerReconciler) deleteHCloudServer(ctx context.Context, obj *infrav1alpha1.HCloudServer) error {
	if obj.Status.ServerID == 0 {
		// No server was ever created.
		return nil
	}
	return r.HCloudClient.DeleteServer(ctx, obj.Status.ServerID)
}

// setCondition updates a condition on the HCloudServer status.
func (r *HCloudServerReconciler) setCondition(
	obj *infrav1alpha1.HCloudServer,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}
