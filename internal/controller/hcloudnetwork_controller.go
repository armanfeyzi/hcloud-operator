package controller

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudNetwork{}).
		Complete(r)
}

func (r *HCloudNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	net := &infrav1alpha1.HCloudNetwork{}
	if err := r.Get(ctx, req.NamespacedName, net); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !net.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(net, hcloudNetworkFinalizer) {
			log.Info("Handling network deletion", "networkID", net.Status.NetworkID)
			if err := r.deleteHCloudNetwork(ctx, net); err != nil {
				return ctrl.Result{}, fmt.Errorf("delete Hetzner network: %w", err)
			}
			controllerutil.RemoveFinalizer(net, hcloudNetworkFinalizer)
			if err := r.Update(ctx, net); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(net, hcloudNetworkFinalizer) {
		controllerutil.AddFinalizer(net, hcloudNetworkFinalizer)
		if err := r.Update(ctx, net); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.reconcileHCloudNetwork(ctx, net); err != nil {
		r.setNetworkCondition(net, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.Status().Update(ctx, net)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
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
		if err := r.Status().Update(ctx, obj); err != nil {
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
	return r.Status().Update(ctx, obj)
}

func subnetZonePresent(zones []string, want string) bool {
	for _, z := range zones {
		if z == want {
			return true
		}
	}
	return false
}

func (r *HCloudNetworkReconciler) deleteHCloudNetwork(ctx context.Context, obj *infrav1alpha1.HCloudNetwork) error {
	if obj.Status.NetworkID == 0 {
		return nil
	}
	return r.HCloudClient.DeleteNetwork(ctx, obj.Status.NetworkID)
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
