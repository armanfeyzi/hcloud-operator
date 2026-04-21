package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
)

const hcloudPlacementGroupFinalizer = "infra.hkc.io/placementgroup-finalizer"

// HCloudPlacementGroupReconciler reconciles HCloudPlacementGroup objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudplacementgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudplacementgroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudplacementgroups/finalizers,verbs=update
type HCloudPlacementGroupReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudPlacementGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudPlacementGroup{}).
		Complete(r)
}

func (r *HCloudPlacementGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	pg := &infrav1alpha1.HCloudPlacementGroup{}
	if err := r.Get(ctx, req.NamespacedName, pg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !pg.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(pg, hcloudPlacementGroupFinalizer) {
			log.Info("Handling placement group deletion", "placementGroupID", pg.Status.PlacementGroupID)
			if err := r.deleteHCloudPlacementGroup(ctx, pg); err != nil {
				return ctrl.Result{}, fmt.Errorf("delete Hetzner placement group: %w", err)
			}
			controllerutil.RemoveFinalizer(pg, hcloudPlacementGroupFinalizer)
			if err := r.Update(ctx, pg); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(pg, hcloudPlacementGroupFinalizer) {
		controllerutil.AddFinalizer(pg, hcloudPlacementGroupFinalizer)
		if err := r.Update(ctx, pg); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.reconcileHCloudPlacementGroup(ctx, pg); err != nil {
		r.setPlacementGroupCondition(pg, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.updatePlacementGroupStatusWithRetry(ctx, pg)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudPlacementGroupReconciler) reconcileHCloudPlacementGroup(ctx context.Context, obj *infrav1alpha1.HCloudPlacementGroup) error {
	log := log.FromContext(ctx)

	var existing *hcloudclient.PlacementGroupInfo
	var err error

	if obj.Status.PlacementGroupID != 0 {
		existing, err = r.HCloudClient.GetPlacementGroup(ctx, obj.Status.PlacementGroupID)
		if err != nil {
			return fmt.Errorf("fetch placement group by ID: %w", err)
		}
	}
	if existing == nil {
		existing, err = r.HCloudClient.GetPlacementGroupByName(ctx, obj.Name)
		if err != nil {
			return fmt.Errorf("fetch placement group by name: %w", err)
		}
	}

	if existing == nil {
		log.Info("Creating new Hetzner placement group", "name", obj.Name, "type", obj.Spec.Type)
		created, err := r.HCloudClient.CreatePlacementGroup(ctx, hcloudclient.PlacementGroupCreateOpts{
			Name:   obj.Name,
			Type:   obj.Spec.Type,
			Labels: obj.Spec.Labels,
		})
		if err != nil {
			return fmt.Errorf("create Hetzner placement group: %w", err)
		}
		obj.Status.PlacementGroupID = created.ID
		obj.Status.Type = created.Type
		r.setPlacementGroupCondition(obj, conditionTypeReady, metav1.ConditionTrue, "PlacementGroupCreated", "Placement group created in Hetzner")
		return r.updatePlacementGroupStatusWithRetry(ctx, obj)
	}

	obj.Status.PlacementGroupID = existing.ID
	obj.Status.Type = existing.Type
	r.setPlacementGroupCondition(obj, conditionTypeReady, metav1.ConditionTrue, "PlacementGroupReady", "Placement group is provisioned")
	return r.updatePlacementGroupStatusWithRetry(ctx, obj)
}

func (r *HCloudPlacementGroupReconciler) deleteHCloudPlacementGroup(ctx context.Context, obj *infrav1alpha1.HCloudPlacementGroup) error {
	if obj.Status.PlacementGroupID == 0 {
		return nil
	}
	return r.HCloudClient.DeletePlacementGroup(ctx, obj.Status.PlacementGroupID)
}

func (r *HCloudPlacementGroupReconciler) setPlacementGroupCondition(
	obj *infrav1alpha1.HCloudPlacementGroup,
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

func (r *HCloudPlacementGroupReconciler) updatePlacementGroupStatusWithRetry(ctx context.Context, obj *infrav1alpha1.HCloudPlacementGroup) error {
	key := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
	desiredStatus := obj.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &infrav1alpha1.HCloudPlacementGroup{}
		if err := r.Get(ctx, key, current); err != nil {
			return err
		}
		current.Status = *desiredStatus.DeepCopy()
		return r.Status().Update(ctx, current)
	})
}
