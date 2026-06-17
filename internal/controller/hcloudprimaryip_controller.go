package controller

import (
	"context"
	"fmt"
	"maps"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
	basereconcile "github.com/armanfeyzi/hcloud-operator/internal/reconcile"
)

const (
	hcloudPrimaryIPFinalizer            = "infra.hkc.io/primaryip-finalizer"
	hcloudPrimaryIPByServerRefNameField = "spec.serverRef.name"
	readyReasonPrimaryIPAssigned        = "PrimaryIPAssigned"
	readyReasonPrimaryIPUnassigned      = "PrimaryIPUnassigned"
)

// HCloudPrimaryIPReconciler reconciles HCloudPrimaryIP objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudprimaryips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudprimaryips/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudprimaryips/finalizers,verbs=update
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch
type HCloudPrimaryIPReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudPrimaryIPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	base := &basereconcile.BaseReconciler[*infrav1alpha1.HCloudPrimaryIP]{
		Client:   r.Client,
		Recorder: mgr.GetEventRecorderFor("hcloud-primaryip"),
		Resource: r,
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &infrav1alpha1.HCloudPrimaryIP{}, hcloudPrimaryIPByServerRefNameField, func(rawObj client.Object) []string {
		pip, ok := rawObj.(*infrav1alpha1.HCloudPrimaryIP)
		if !ok || pip.Spec.ServerRef == nil || pip.Spec.ServerRef.Name == "" {
			return nil
		}
		return []string{pip.Spec.ServerRef.Name}
	}); err != nil {
		return fmt.Errorf("index HCloudPrimaryIP by serverRef name: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudPrimaryIP{}).
		Watches(
			&infrav1alpha1.HCloudServer{},
			handler.EnqueueRequestsFromMapFunc(r.mapServerToPrimaryIPs),
		).
		Complete(base)
}

func (r *HCloudPrimaryIPReconciler) NewObject() *infrav1alpha1.HCloudPrimaryIP {
	return &infrav1alpha1.HCloudPrimaryIP{}
}

func (r *HCloudPrimaryIPReconciler) FinalizerName() string { return hcloudPrimaryIPFinalizer }

func (r *HCloudPrimaryIPReconciler) Kind() string { return "HCloudPrimaryIP" }

func (r *HCloudPrimaryIPReconciler) mapServerToPrimaryIPs(ctx context.Context, obj client.Object) []reconcile.Request {
	server, ok := obj.(*infrav1alpha1.HCloudServer)
	if !ok {
		return nil
	}

	var primaryIPs infrav1alpha1.HCloudPrimaryIPList
	if err := r.List(ctx, &primaryIPs, client.MatchingFields{hcloudPrimaryIPByServerRefNameField: server.Name}); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(primaryIPs.Items))
	for i := range primaryIPs.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: primaryIPs.Items[i].Name}})
	}
	return requests
}

func (r *HCloudPrimaryIPReconciler) Reconcile(ctx context.Context, pip *infrav1alpha1.HCloudPrimaryIP) (ctrl.Result, error) {
	targetServerID, pending, err := r.resolveTargetServerID(ctx, pip)
	if err != nil {
		return ctrl.Result{}, err
	}
	if pending {
		r.setPrimaryIPCondition(pip, conditionTypeReady, metav1.ConditionFalse, "ServerPending", "Waiting for referenced HCloudServer to be provisioned")
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	if err := r.reconcileHCloudPrimaryIP(ctx, pip, targetServerID); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudPrimaryIPReconciler) Delete(ctx context.Context, pip *infrav1alpha1.HCloudPrimaryIP) error {
	if pip.Status.PrimaryIPID == 0 {
		return nil
	}
	if err := r.HCloudClient.UnassignPrimaryIP(ctx, pip.Status.PrimaryIPID); err != nil {
		return fmt.Errorf("unassign primary IP before delete: %w", err)
	}
	return r.HCloudClient.DeletePrimaryIP(ctx, pip.Status.PrimaryIPID)
}

func (r *HCloudPrimaryIPReconciler) resolveTargetServerID(ctx context.Context, obj *infrav1alpha1.HCloudPrimaryIP) (int64, bool, error) {
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

func (r *HCloudPrimaryIPReconciler) reconcileHCloudPrimaryIP(ctx context.Context, obj *infrav1alpha1.HCloudPrimaryIP, targetServerID int64) error {
	log := log.FromContext(ctx)

	var existing *hcloudclient.PrimaryIPInfo
	var err error

	if obj.Status.PrimaryIPID != 0 {
		existing, err = r.HCloudClient.GetPrimaryIP(ctx, obj.Status.PrimaryIPID)
		if err != nil {
			return fmt.Errorf("fetch primary IP by ID: %w", err)
		}
	}
	if existing == nil {
		existing, err = r.HCloudClient.GetPrimaryIPByName(ctx, obj.Name)
		if err != nil {
			return fmt.Errorf("fetch primary IP by name: %w", err)
		}
	}

	if existing == nil {
		log.Info("Creating new Hetzner primary IP", "name", obj.Name, "type", obj.Spec.Type, "datacenter", obj.Spec.Datacenter)
		// Create without assignee: Hetzner rejects create when both datacenter and assignee_id are set.
		// Assignment is handled below by ensurePrimaryIPAssignment.
		created, err := r.HCloudClient.CreatePrimaryIP(ctx, hcloudclient.PrimaryIPCreateOpts{
			Name:       obj.Name,
			Type:       obj.Spec.Type,
			Datacenter: obj.Spec.Datacenter,
			Labels:     obj.Spec.Labels,
			AutoDelete: obj.Spec.AutoDelete,
		})
		if err != nil {
			return fmt.Errorf("create Hetzner primary IP: %w", err)
		}
		existing = created
	}

	if err := r.ensurePrimaryIPMetadata(ctx, obj, existing); err != nil {
		return err
	}

	refreshed, err := r.HCloudClient.GetPrimaryIP(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("refresh primary IP: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("primary IP %d disappeared after metadata sync", existing.ID)
	}
	existing = refreshed

	if err := r.ensurePrimaryIPAssignment(ctx, obj, existing, targetServerID); err != nil {
		return err
	}

	refreshed, err = r.HCloudClient.GetPrimaryIP(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("refresh primary IP after assignment: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("primary IP %d disappeared after assignment", existing.ID)
	}
	existing = refreshed

	if err := r.ensurePrimaryIPDNSPtr(ctx, obj, existing); err != nil {
		return err
	}

	refreshed, err = r.HCloudClient.GetPrimaryIP(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("refresh primary IP after DNS update: %w", err)
	}
	if refreshed == nil {
		return fmt.Errorf("primary IP %d disappeared after DNS update", existing.ID)
	}

	r.syncPrimaryIPStatus(obj, refreshed)
	r.setPrimaryIPCondition(obj, conditionTypeReady, metav1.ConditionTrue, "PrimaryIPReady", "Primary IP is provisioned")
	return nil
}

func (r *HCloudPrimaryIPReconciler) ensurePrimaryIPMetadata(ctx context.Context, obj *infrav1alpha1.HCloudPrimaryIP, existing *hcloudclient.PrimaryIPInfo) error {
	update := hcloudclient.PrimaryIPUpdateOpts{}
	labelsChanged := !mapsEqual(existing.Labels, obj.Spec.Labels)
	if labelsChanged {
		update.Labels = cloneStringMapController(obj.Spec.Labels)
	}
	autoDeleteChanged := obj.Spec.AutoDelete != nil && existing.AutoDelete != *obj.Spec.AutoDelete
	if autoDeleteChanged {
		update.AutoDelete = obj.Spec.AutoDelete
	}
	if labelsChanged || autoDeleteChanged {
		if err := r.HCloudClient.UpdatePrimaryIP(ctx, existing.ID, update); err != nil {
			return fmt.Errorf("update primary IP metadata: %w", err)
		}
	}
	return nil
}

func (r *HCloudPrimaryIPReconciler) ensurePrimaryIPAssignment(
	ctx context.Context,
	obj *infrav1alpha1.HCloudPrimaryIP,
	existing *hcloudclient.PrimaryIPInfo,
	targetServerID int64,
) error {
	if targetServerID == 0 {
		if obj.Status.AppliedAssigneeID != 0 && existing.AssigneeID == obj.Status.AppliedAssigneeID {
			if err := r.HCloudClient.UnassignPrimaryIP(ctx, existing.ID); err != nil {
				return fmt.Errorf("unassign primary IP %d: %w", existing.ID, err)
			}
			obj.Status.AppliedAssigneeID = 0
			r.setPrimaryIPCondition(obj, conditionTypeReady, metav1.ConditionTrue, readyReasonPrimaryIPUnassigned, "Primary IP unassigned from server")
		}
		return nil
	}

	if existing.AssigneeID == targetServerID {
		obj.Status.AppliedAssigneeID = targetServerID
		return nil
	}

	if existing.AssigneeID != 0 && existing.AssigneeID != targetServerID {
		if err := r.HCloudClient.UnassignPrimaryIP(ctx, existing.ID); err != nil {
			return fmt.Errorf("unassign primary IP %d before reassignment: %w", existing.ID, err)
		}
	}

	if err := r.HCloudClient.AssignPrimaryIP(ctx, existing.ID, targetServerID, "server"); err != nil {
		return fmt.Errorf("assign primary IP %d to server %d: %w", existing.ID, targetServerID, err)
	}
	obj.Status.AppliedAssigneeID = targetServerID
	r.setPrimaryIPCondition(obj, conditionTypeReady, metav1.ConditionTrue, readyReasonPrimaryIPAssigned, fmt.Sprintf("Primary IP assigned to server ID %d", targetServerID))
	return nil
}

func (r *HCloudPrimaryIPReconciler) ensurePrimaryIPDNSPtr(ctx context.Context, obj *infrav1alpha1.HCloudPrimaryIP, existing *hcloudclient.PrimaryIPInfo) error {
	if obj.Spec.DNSPtr == nil || existing.IP == "" {
		return nil
	}
	current := existing.DNSPtr[existing.IP]
	if current == *obj.Spec.DNSPtr {
		return nil
	}
	if err := r.HCloudClient.ChangePrimaryIPDNSPtr(ctx, existing.ID, existing.IP, *obj.Spec.DNSPtr); err != nil {
		return fmt.Errorf("change primary IP DNS PTR: %w", err)
	}
	return nil
}

func (r *HCloudPrimaryIPReconciler) syncPrimaryIPStatus(obj *infrav1alpha1.HCloudPrimaryIP, existing *hcloudclient.PrimaryIPInfo) {
	obj.Status.PrimaryIPID = existing.ID
	obj.Status.IP = existing.IP
	obj.Status.Datacenter = existing.Datacenter
	if existing.AssigneeID != 0 {
		obj.Status.AppliedAssigneeID = existing.AssigneeID
	}
}

func (r *HCloudPrimaryIPReconciler) setPrimaryIPCondition(
	obj *infrav1alpha1.HCloudPrimaryIP,
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

func mapsEqual(a, b map[string]string) bool {
	return maps.Equal(a, b)
}

func cloneStringMapController(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
