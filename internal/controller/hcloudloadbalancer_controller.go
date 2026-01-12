package controller

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
)

const hcloudLoadBalancerFinalizer = "infra.hkc.io/loadbalancer-finalizer"

// HCloudLoadBalancerReconciler reconciles HCloudLoadBalancer objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudloadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudloadbalancers/finalizers,verbs=update
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch
type HCloudLoadBalancerReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudLoadBalancer{}).
		Complete(r)
}

func (r *HCloudLoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lb := &infrav1alpha1.HCloudLoadBalancer{}
	if err := r.Get(ctx, req.NamespacedName, lb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !lb.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(lb, hcloudLoadBalancerFinalizer) {
			if lb.Status.LoadBalancerID != 0 {
				if err := r.HCloudClient.DeleteLoadBalancer(ctx, lb.Status.LoadBalancerID); err != nil {
					return ctrl.Result{}, fmt.Errorf("delete load balancer: %w", err)
				}
			}
			controllerutil.RemoveFinalizer(lb, hcloudLoadBalancerFinalizer)
			if err := r.Update(ctx, lb); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(lb, hcloudLoadBalancerFinalizer) {
		controllerutil.AddFinalizer(lb, hcloudLoadBalancerFinalizer)
		if err := r.Update(ctx, lb); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	selectedServerIDs, err := r.getSelectedServerIDs(ctx, lb.Spec.ServerSelector)
	if err != nil {
		r.setCondition(lb, conditionTypeReady, metav1.ConditionFalse, "SelectorError", err.Error())
		_ = r.Status().Update(ctx, lb)
		return ctrl.Result{}, err
	}

	if err := r.reconcileLoadBalancer(ctx, lb, selectedServerIDs); err != nil {
		r.setCondition(lb, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.Status().Update(ctx, lb)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudLoadBalancerReconciler) reconcileLoadBalancer(
	ctx context.Context,
	obj *infrav1alpha1.HCloudLoadBalancer,
	selectedServerIDs []int64,
) error {
	var existing *hcloudclient.LoadBalancerInfo
	var err error

	if obj.Status.LoadBalancerID != 0 {
		existing, err = r.HCloudClient.GetLoadBalancer(ctx, obj.Status.LoadBalancerID)
		if err != nil {
			return fmt.Errorf("fetch load balancer by ID: %w", err)
		}
	}
	if existing == nil {
		existing, err = r.HCloudClient.GetLoadBalancerByName(ctx, obj.Name)
		if err != nil {
			return fmt.Errorf("fetch load balancer by name: %w", err)
		}
	}
	if existing == nil {
		created, err := r.HCloudClient.CreateLoadBalancer(ctx, hcloudclient.LoadBalancerCreateOpts{
			Name:             obj.Name,
			LoadBalancerType: obj.Spec.LoadBalancerType,
			Location:         obj.Spec.Location,
			NetworkZone:      obj.Spec.NetworkZone,
			Algorithm:        obj.Spec.Algorithm,
			Labels:           obj.Spec.Labels,
		})
		if err != nil {
			return fmt.Errorf("create load balancer: %w", err)
		}
		existing = created
	}

	currentTargets := map[int64]struct{}{}
	for _, id := range existing.Targets {
		currentTargets[id] = struct{}{}
	}
	selectedTargets := map[int64]struct{}{}
	for _, id := range selectedServerIDs {
		selectedTargets[id] = struct{}{}
	}

	for id := range selectedTargets {
		if _, ok := currentTargets[id]; ok {
			continue
		}
		if err := r.HCloudClient.AttachServerToLoadBalancer(ctx, existing.ID, id); err != nil {
			return fmt.Errorf("attach server %d to load balancer %d: %w", id, existing.ID, err)
		}
	}
	for id := range currentTargets {
		if _, ok := selectedTargets[id]; ok {
			continue
		}
		if err := r.HCloudClient.DetachServerFromLoadBalancer(ctx, existing.ID, id); err != nil {
			return fmt.Errorf("detach server %d from load balancer %d: %w", id, existing.ID, err)
		}
	}

	refreshed, err := r.HCloudClient.GetLoadBalancer(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("refresh load balancer: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("load balancer %d disappeared after reconcile", existing.ID)
	}

	slices.Sort(refreshed.Targets)
	obj.Status.LoadBalancerID = refreshed.ID
	obj.Status.PublicIPv4 = refreshed.PublicIPv4
	obj.Status.PublicIPv6 = refreshed.PublicIPv6
	obj.Status.AttachedServerIDs = refreshed.Targets
	r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, "LoadBalancerReady", "Load balancer is provisioned and targets are in sync")
	return r.Status().Update(ctx, obj)
}

func (r *HCloudLoadBalancerReconciler) getSelectedServerIDs(ctx context.Context, selector *metav1.LabelSelector) ([]int64, error) {
	servers := &infrav1alpha1.HCloudServerList{}
	listOpts := []client.ListOption{}
	var labelSelector labels.Selector
	if selector != nil {
		compiled, err := metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			return nil, fmt.Errorf("invalid serverSelector: %w", err)
		}
		labelSelector = compiled
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: compiled})
	}
	if err := r.List(ctx, servers, listOpts...); err != nil {
		return nil, fmt.Errorf("list HCloudServer resources: %w", err)
	}

	selected := make([]int64, 0, len(servers.Items))
	for _, server := range servers.Items {
		if server.Status.ServerID == 0 {
			continue
		}
		if selector != nil {
			if !labelSelector.Matches(labels.Set(server.Labels)) {
				continue
			}
		}
		selected = append(selected, server.Status.ServerID)
	}
	slices.Sort(selected)
	return selected, nil
}

func (r *HCloudLoadBalancerReconciler) setCondition(
	obj *infrav1alpha1.HCloudLoadBalancer,
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
