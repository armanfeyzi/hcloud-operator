package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
	basereconcile "github.com/armanfeyzi/hcloud-operator/internal/reconcile"
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
	base := &basereconcile.BaseReconciler[*infrav1alpha1.HCloudPlacementGroup]{
		Client:   r.Client,
		Recorder: mgr.GetEventRecorderFor("hcloud-placementgroup"),
		Resource: r,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudPlacementGroup{}).
		Complete(base)
}

func (r *HCloudPlacementGroupReconciler) NewObject() *infrav1alpha1.HCloudPlacementGroup {
	return &infrav1alpha1.HCloudPlacementGroup{}
}

func (r *HCloudPlacementGroupReconciler) FinalizerName() string { return hcloudPlacementGroupFinalizer }

func (r *HCloudPlacementGroupReconciler) Kind() string { return "HCloudPlacementGroup" }

func (r *HCloudPlacementGroupReconciler) Reconcile(ctx context.Context, pg *infrav1alpha1.HCloudPlacementGroup) (ctrl.Result, error) {
	if err := r.reconcileHCloudPlacementGroup(ctx, pg); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudPlacementGroupReconciler) Delete(ctx context.Context, pg *infrav1alpha1.HCloudPlacementGroup) error {
	if pg.Status.PlacementGroupID == 0 {
		return nil
	}
	return r.HCloudClient.DeletePlacementGroup(ctx, pg.Status.PlacementGroupID)
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
		return nil
	}

	obj.Status.PlacementGroupID = existing.ID
	obj.Status.Type = existing.Type
	r.setPlacementGroupCondition(obj, conditionTypeReady, metav1.ConditionTrue, "PlacementGroupReady", "Placement group is provisioned")
	return nil
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
