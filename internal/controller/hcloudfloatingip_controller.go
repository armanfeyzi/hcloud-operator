package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
)

const (
	hcloudFloatingIPFinalizer            = "infra.hkc.io/floatingip-finalizer"
	hcloudFloatingIPByServerRefNameField = "spec.serverRef.name"
	readyReasonFloatingIPAssigned        = "FloatingIPAssigned"
	readyReasonFloatingIPUnassigned      = "FloatingIPUnassigned"
)

// HCloudFloatingIPReconciler reconciles HCloudFloatingIP objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudfloatingips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudfloatingips/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudfloatingips/finalizers,verbs=update
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch
type HCloudFloatingIPReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudFloatingIPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &infrav1alpha1.HCloudFloatingIP{}, hcloudFloatingIPByServerRefNameField, func(rawObj client.Object) []string {
		fip, ok := rawObj.(*infrav1alpha1.HCloudFloatingIP)
		if !ok || fip.Spec.ServerRef == nil || fip.Spec.ServerRef.Name == "" {
			return nil
		}
		return []string{fip.Spec.ServerRef.Name}
	}); err != nil {
		return fmt.Errorf("index HCloudFloatingIP by serverRef name: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudFloatingIP{}).
		Watches(
			&infrav1alpha1.HCloudServer{},
			handler.EnqueueRequestsFromMapFunc(r.mapServerToFloatingIPs),
		).
		Complete(r)
}

func (r *HCloudFloatingIPReconciler) mapServerToFloatingIPs(ctx context.Context, obj client.Object) []reconcile.Request {
	server, ok := obj.(*infrav1alpha1.HCloudServer)
	if !ok {
		return nil
	}

	var floatingIPs infrav1alpha1.HCloudFloatingIPList
	if err := r.List(ctx, &floatingIPs, client.MatchingFields{hcloudFloatingIPByServerRefNameField: server.Name}); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(floatingIPs.Items))
	for i := range floatingIPs.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: floatingIPs.Items[i].Name}})
	}
	return requests
}

func (r *HCloudFloatingIPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	fip := &infrav1alpha1.HCloudFloatingIP{}
	if err := r.Get(ctx, req.NamespacedName, fip); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !fip.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(fip, hcloudFloatingIPFinalizer) {
			log.Info("Handling floating IP deletion", "floatingIPID", fip.Status.FloatingIPID)
			if err := r.deleteHCloudFloatingIP(ctx, fip); err != nil {
				return ctrl.Result{}, fmt.Errorf("delete Hetzner floating IP: %w", err)
			}
			controllerutil.RemoveFinalizer(fip, hcloudFloatingIPFinalizer)
			if err := r.Update(ctx, fip); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(fip, hcloudFloatingIPFinalizer) {
		controllerutil.AddFinalizer(fip, hcloudFloatingIPFinalizer)
		if err := r.Update(ctx, fip); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	targetServerID, pending, err := r.resolveTargetServerID(ctx, fip)
	if err != nil {
		r.setFloatingIPCondition(fip, conditionTypeReady, metav1.ConditionFalse, "ServerRefError", err.Error())
		_ = r.updateFloatingIPStatusWithRetry(ctx, fip)
		return ctrl.Result{}, err
	}
	if pending {
		r.setFloatingIPCondition(fip, conditionTypeReady, metav1.ConditionFalse, "ServerPending", "Waiting for referenced HCloudServer to be provisioned")
		_ = r.updateFloatingIPStatusWithRetry(ctx, fip)
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	if err := r.reconcileHCloudFloatingIP(ctx, fip, targetServerID); err != nil {
		r.setFloatingIPCondition(fip, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.updateFloatingIPStatusWithRetry(ctx, fip)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudFloatingIPReconciler) resolveTargetServerID(ctx context.Context, obj *infrav1alpha1.HCloudFloatingIP) (int64, bool, error) {
	if obj.Spec.ServerRef == nil || obj.Spec.ServerRef.Name == "" {
		return 0, false, nil
	}

	server := &infrav1alpha1.HCloudServer{}
	if err := r.Get(ctx, client.ObjectKey{Name: obj.Spec.ServerRef.Name}, server); err != nil {
		if apierrors.IsNotFound(err) {
			return 0, true, nil
		}
		return 0, false, fmt.Errorf("get referenced HCloudServer %q: %w", obj.Spec.ServerRef.Name, err)
	}
	if server.Status.ServerID == 0 {
		return 0, true, nil
	}
	return server.Status.ServerID, false, nil
}

func (r *HCloudFloatingIPReconciler) reconcileHCloudFloatingIP(ctx context.Context, obj *infrav1alpha1.HCloudFloatingIP, targetServerID int64) error {
	log := log.FromContext(ctx)

	var existing *hcloudclient.FloatingIPInfo
	var err error

	if obj.Status.FloatingIPID != 0 {
		existing, err = r.HCloudClient.GetFloatingIP(ctx, obj.Status.FloatingIPID)
		if err != nil {
			return fmt.Errorf("fetch floating IP by ID: %w", err)
		}
	}
	if existing == nil {
		existing, err = r.HCloudClient.GetFloatingIPByName(ctx, obj.Name)
		if err != nil {
			return fmt.Errorf("fetch floating IP by name: %w", err)
		}
	}

	if existing == nil {
		log.Info("Creating new Hetzner floating IP", "name", obj.Name, "type", obj.Spec.Type, "location", obj.Spec.Location)
		created, err := r.HCloudClient.CreateFloatingIP(ctx, hcloudclient.FloatingIPCreateOpts{
			Name:        obj.Name,
			Type:        obj.Spec.Type,
			Location:    obj.Spec.Location,
			Labels:      obj.Spec.Labels,
			Description: obj.Spec.Description,
			ServerID:    targetServerID,
		})
		if err != nil {
			return fmt.Errorf("create Hetzner floating IP: %w", err)
		}
		existing = created
	}

	if err := r.ensureFloatingIPMetadata(ctx, obj, existing); err != nil {
		return err
	}

	refreshed, err := r.HCloudClient.GetFloatingIP(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("refresh floating IP: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("floating IP %d disappeared after metadata sync", existing.ID)
	}
	existing = refreshed

	if err := r.ensureFloatingIPAssignment(ctx, obj, existing, targetServerID); err != nil {
		return err
	}

	refreshed, err = r.HCloudClient.GetFloatingIP(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("refresh floating IP after assignment: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("floating IP %d disappeared after assignment", existing.ID)
	}
	existing = refreshed

	if err := r.ensureFloatingIPDNSPtr(ctx, obj, existing); err != nil {
		return err
	}

	refreshed, err = r.HCloudClient.GetFloatingIP(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("refresh floating IP after DNS update: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("floating IP %d disappeared after DNS update", existing.ID)
	}

	r.syncFloatingIPStatus(obj, refreshed)
	r.setFloatingIPCondition(obj, conditionTypeReady, metav1.ConditionTrue, "FloatingIPReady", "Floating IP is provisioned")
	return r.updateFloatingIPStatusWithRetry(ctx, obj)
}

func (r *HCloudFloatingIPReconciler) ensureFloatingIPMetadata(ctx context.Context, obj *infrav1alpha1.HCloudFloatingIP, existing *hcloudclient.FloatingIPInfo) error {
	update := hcloudclient.FloatingIPUpdateOpts{}
	labelsChanged := !mapsEqual(existing.Labels, obj.Spec.Labels)
	if labelsChanged {
		update.Labels = cloneStringMapController(obj.Spec.Labels)
	}
	descriptionChanged := existing.Description != obj.Spec.Description
	if descriptionChanged {
		update.Description = obj.Spec.Description
	}
	if labelsChanged || descriptionChanged {
		if err := r.HCloudClient.UpdateFloatingIP(ctx, existing.ID, update); err != nil {
			return fmt.Errorf("update floating IP metadata: %w", err)
		}
	}
	return nil
}

func (r *HCloudFloatingIPReconciler) ensureFloatingIPAssignment(
	ctx context.Context,
	obj *infrav1alpha1.HCloudFloatingIP,
	existing *hcloudclient.FloatingIPInfo,
	targetServerID int64,
) error {
	if targetServerID == 0 {
		if obj.Status.AppliedServerID != 0 && existing.ServerID == obj.Status.AppliedServerID {
			if err := r.HCloudClient.UnassignFloatingIP(ctx, existing.ID); err != nil {
				return fmt.Errorf("unassign floating IP %d: %w", existing.ID, err)
			}
			obj.Status.AppliedServerID = 0
			r.setFloatingIPCondition(obj, conditionTypeReady, metav1.ConditionTrue, readyReasonFloatingIPUnassigned, "Floating IP unassigned from server")
		}
		return nil
	}

	if existing.ServerID == targetServerID {
		obj.Status.AppliedServerID = targetServerID
		return nil
	}

	if existing.ServerID != 0 && existing.ServerID != targetServerID {
		if err := r.HCloudClient.UnassignFloatingIP(ctx, existing.ID); err != nil {
			return fmt.Errorf("unassign floating IP %d before reassignment: %w", existing.ID, err)
		}
	}

	if err := r.HCloudClient.AssignFloatingIP(ctx, existing.ID, targetServerID); err != nil {
		return fmt.Errorf("assign floating IP %d to server %d: %w", existing.ID, targetServerID, err)
	}
	obj.Status.AppliedServerID = targetServerID
	r.setFloatingIPCondition(obj, conditionTypeReady, metav1.ConditionTrue, readyReasonFloatingIPAssigned, fmt.Sprintf("Floating IP assigned to server ID %d", targetServerID))
	return nil
}

func (r *HCloudFloatingIPReconciler) ensureFloatingIPDNSPtr(ctx context.Context, obj *infrav1alpha1.HCloudFloatingIP, existing *hcloudclient.FloatingIPInfo) error {
	if obj.Spec.DNSPtr == nil || existing.IP == "" {
		return nil
	}
	current := existing.DNSPtr[existing.IP]
	if current == *obj.Spec.DNSPtr {
		return nil
	}
	if err := r.HCloudClient.ChangeFloatingIPDNSPtr(ctx, existing.ID, existing.IP, *obj.Spec.DNSPtr); err != nil {
		return fmt.Errorf("change floating IP DNS PTR: %w", err)
	}
	return nil
}

func (r *HCloudFloatingIPReconciler) syncFloatingIPStatus(obj *infrav1alpha1.HCloudFloatingIP, existing *hcloudclient.FloatingIPInfo) {
	obj.Status.FloatingIPID = existing.ID
	obj.Status.IP = existing.IP
	obj.Status.Location = existing.Location
	if existing.ServerID != 0 {
		obj.Status.AppliedServerID = existing.ServerID
	}
}

func (r *HCloudFloatingIPReconciler) deleteHCloudFloatingIP(ctx context.Context, obj *infrav1alpha1.HCloudFloatingIP) error {
	if obj.Status.FloatingIPID == 0 {
		return nil
	}
	if err := r.HCloudClient.UnassignFloatingIP(ctx, obj.Status.FloatingIPID); err != nil {
		return fmt.Errorf("unassign floating IP before delete: %w", err)
	}
	return r.HCloudClient.DeleteFloatingIP(ctx, obj.Status.FloatingIPID)
}

func (r *HCloudFloatingIPReconciler) setFloatingIPCondition(
	obj *infrav1alpha1.HCloudFloatingIP,
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

func (r *HCloudFloatingIPReconciler) updateFloatingIPStatusWithRetry(ctx context.Context, obj *infrav1alpha1.HCloudFloatingIP) error {
	key := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
	desiredStatus := obj.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &infrav1alpha1.HCloudFloatingIP{}
		if err := r.Get(ctx, key, current); err != nil {
			return err
		}
		current.Status = *desiredStatus.DeepCopy()
		return r.Status().Update(ctx, current)
	})
}
