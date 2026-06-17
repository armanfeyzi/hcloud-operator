package controller

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
	basereconcile "github.com/armanfeyzi/hcloud-operator/internal/reconcile"
)

const hcloudNetworkFinalizer = "infra.hkc.io/network-finalizer"

// HCloudNetworkReconciler reconciles HCloudNetwork objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudnetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudnetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudnetworks/finalizers,verbs=update
type HCloudNetworkReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	base := &basereconcile.BaseReconciler[*infrav1alpha1.HCloudNetwork]{
		Client:   r.Client,
		Recorder: mgr.GetEventRecorderFor("hcloud-network"),
		Resource: r,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudNetwork{}).
		Complete(base)
}

func (r *HCloudNetworkReconciler) NewObject() *infrav1alpha1.HCloudNetwork {
	return &infrav1alpha1.HCloudNetwork{}
}

func (r *HCloudNetworkReconciler) FinalizerName() string { return hcloudNetworkFinalizer }

func (r *HCloudNetworkReconciler) Kind() string { return "HCloudNetwork" }

func (r *HCloudNetworkReconciler) Reconcile(ctx context.Context, net *infrav1alpha1.HCloudNetwork) (ctrl.Result, error) {
	if err := r.reconcileHCloudNetwork(ctx, net); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudNetworkReconciler) Delete(ctx context.Context, net *infrav1alpha1.HCloudNetwork) error {
	if net.Status.NetworkID == 0 {
		return nil
	}
	return r.HCloudClient.DeleteNetwork(ctx, net.Status.NetworkID)
}

func (r *HCloudNetworkReconciler) reconcileHCloudNetwork(ctx context.Context, obj *infrav1alpha1.HCloudNetwork) error {
	log := log.FromContext(ctx)

	var existing *hcloudclient.NetworkInfo
	var err error

	if obj.Status.NetworkID != 0 {
		existing, err = r.HCloudClient.GetNetwork(ctx, obj.Status.NetworkID)
		if err != nil {
			return fmt.Errorf("fetch network by ID: %w", err)
		}
	}
	if existing == nil {
		existing, err = r.HCloudClient.GetNetworkByName(ctx, obj.Name)
		if err != nil {
			return fmt.Errorf("fetch network by name: %w", err)
		}
	}

	if existing == nil {
		log.Info("Creating new Hetzner private network", "name", obj.Name, "ipRange", obj.Spec.IPRange)
		created, err := r.HCloudClient.CreateNetwork(ctx, hcloudclient.NetworkCreateOpts{
			Name:                  obj.Name,
			IPRange:               obj.Spec.IPRange,
			Labels:                obj.Spec.Labels,
			ExposeRoutesToVSwitch: obj.Spec.ExposeRoutesToVSwitch,
		})
		if err != nil {
			return fmt.Errorf("create Hetzner network: %w", err)
		}
		obj.Status.NetworkID = created.ID
		obj.Status.IPRange = created.IPRange
		obj.Status.SubnetZones = append([]string{}, created.SubnetZones...)
		r.setNetworkCondition(obj, conditionTypeReady, metav1.ConditionFalse, "NetworkCreated", "Private network created; adding subnets if requested")
		if err := r.updateNetworkStatusWithRetry(ctx, obj); err != nil {
			return fmt.Errorf("persist network id: %w", err)
		}
		existing = created
	}

	for _, zone := range obj.Spec.NetworkZones {
		if subnetZonePresent(existing.SubnetZones, zone) {
			continue
		}
		log.Info("Adding Cloud subnet to network", "networkID", existing.ID, "zone", zone)
		if err := r.HCloudClient.AddNetworkCloudSubnet(ctx, existing.ID, zone); err != nil {
			return fmt.Errorf("add subnet in zone %q: %w", zone, err)
		}
		refreshed, err := r.HCloudClient.GetNetwork(ctx, existing.ID)
		if err != nil {
			return fmt.Errorf("refresh network after add subnet: %w", err)
		}
		if refreshed == nil {
			return fmt.Errorf("network %d disappeared after add subnet", existing.ID)
		}
		existing = refreshed
	}

	obj.Status.NetworkID = existing.ID
	obj.Status.IPRange = existing.IPRange
	obj.Status.SubnetZones = append([]string{}, existing.SubnetZones...)
	slices.Sort(obj.Status.SubnetZones)
	r.setNetworkCondition(obj, conditionTypeReady, metav1.ConditionTrue, "NetworkReady", "Private network is provisioned")
	return nil
}

func subnetZonePresent(zones []string, want string) bool {
	for _, z := range zones {
		if z == want {
			return true
		}
	}
	return false
}

func (r *HCloudNetworkReconciler) setNetworkCondition(
	obj *infrav1alpha1.HCloudNetwork,
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

func (r *HCloudNetworkReconciler) updateNetworkStatusWithRetry(ctx context.Context, obj *infrav1alpha1.HCloudNetwork) error {
	key := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
	desiredStatus := obj.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &infrav1alpha1.HCloudNetwork{}
		if err := r.Get(ctx, key, current); err != nil {
			return err
		}
		current.Status = *desiredStatus.DeepCopy()
		return r.Status().Update(ctx, current)
	})
}
