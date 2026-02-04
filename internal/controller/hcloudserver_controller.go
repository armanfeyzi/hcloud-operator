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

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
)

const (
	// hcloudServerFinalizer is the finalizer added to HCloudServer resources.
	// The controller will not allow deletion until Hetzner-side cleanup is complete.
	hcloudServerFinalizer = "infra.hkc.io/finalizer"

	// requeueDelay is the default requeue interval for non-error reconciliations.
	requeueDelay = 30 * time.Second

	// resizeRequeueDelay is used while waiting for power actions or type changes to complete.
	resizeRequeueDelay = 5 * time.Second

	// conditionTypeReady is the condition type used to indicate resource readiness.
	conditionTypeReady         = "Ready"
	readyReasonNetworkAttached = "NetworkAttached"
	readyReasonNetworkMigrated = "NetworkMigrated"
	readyReasonNetworkDetached = "NetworkDetached"
)

// HCloudServerReconciler reconciles HCloudServer objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers/finalizers,verbs=update
type HCloudServerReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
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
	requeueAfter, err := r.reconcileHCloudServer(ctx, server)
	if err != nil {
		r.setCondition(server, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.Status().Update(ctx, server)
		return ctrl.Result{}, err
	}

	delay := requeueDelay
	if requeueAfter > 0 {
		delay = requeueAfter
	}
	return ctrl.Result{RequeueAfter: delay}, nil
}

// reconcileHCloudServer ensures the Hetzner Cloud server matches the desired spec.
// A positive return value is a custom requeue interval (e.g. while resizing); zero means use the default.
func (r *HCloudServerReconciler) reconcileHCloudServer(ctx context.Context, obj *infrav1alpha1.HCloudServer) (time.Duration, error) {
	log := log.FromContext(ctx)

	// If we have a stored server ID, try fetching by ID first.
	if obj.Status.ServerID != 0 {
		existing, err := r.HCloudClient.GetServer(ctx, obj.Status.ServerID)
		if err != nil {
			return 0, fmt.Errorf("fetch server by ID: %w", err)
		}
		if existing != nil {
			log.Info("Server exists, reconciling", "serverID", existing.ID)
			return r.reconcileExistingServer(ctx, obj, existing)
		}
		// Server was deleted externally — fall through to name-based lookup.
		log.Info("Server not found by stored ID, checking by name", "serverID", obj.Status.ServerID)
	}

	// Before creating, look up by name. This handles the case where a previous
	// reconcile created the server in Hetzner but crashed before writing the ID
	// back to status — without this check we'd spin up a duplicate every time.
	existing, err := r.HCloudClient.GetServerByName(ctx, obj.Name)
	if err != nil {
		return 0, fmt.Errorf("fetch server by name: %w", err)
	}
	if existing != nil {
		log.Info("Adopting existing Hetzner server found by name", "serverID", existing.ID)
		obj.Status.ServerID = existing.ID
		return r.reconcileExistingServer(ctx, obj, existing)
	}

	// No server found — create one.
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
		return 0, fmt.Errorf("create Hetzner server: %w", err)
	}

	obj.Status.ServerID = created.ID
	obj.Status.State = created.State
	obj.Status.PublicIPv4 = created.PublicIPv4
	obj.Status.PublicIPv6 = created.PublicIPv6
	if created.ServerType == obj.Spec.ServerType && created.State == "running" {
		obj.Status.AppliedServerType = obj.Spec.ServerType
	}
	r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, "ServerCreated", "Hetzner server created successfully")

	return 0, r.Status().Update(ctx, obj)
}

func (r *HCloudServerReconciler) reconcileExistingServer(ctx context.Context, obj *infrav1alpha1.HCloudServer, s *hcloudclient.ServerInfo) (time.Duration, error) {
	desired := obj.Spec.ServerType
	applied := obj.Status.AppliedServerType

	// Only drive the resize state machine when we have an observed type from Hetzner that differs from spec.
	if s.ServerType != "" && s.ServerType != desired {
		return r.reconcileServerTypeMismatch(ctx, obj, s)
	}

	// Types match Hetzner; power on only while finishing a spec-driven type change
	// (AppliedServerType lags until the server is running again).
	if applied != desired && s.State == "off" {
		if err := r.HCloudClient.PowerOnServer(ctx, s.ID); err != nil {
			return 0, err
		}
		s2, err := r.HCloudClient.GetServer(ctx, s.ID)
		if err != nil {
			return 0, fmt.Errorf("refresh server after power on: %w", err)
		}
		r.applyServerStatus(obj, s2)
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "PoweringOn", "Powered on server after type change; waiting for running state")
		return resizeRequeueDelay, r.Status().Update(ctx, obj)
	}

	networkChanged, err := r.ensureServerNetworkAttachment(ctx, obj, s)
	if err != nil {
		return 0, err
	}
	if networkChanged {
		r.applyServerStatus(obj, s)
		return resizeRequeueDelay, r.Status().Update(ctx, obj)
	}

	return 0, r.syncStatus(ctx, obj, s)
}

func (r *HCloudServerReconciler) ensureServerNetworkAttachment(
	ctx context.Context,
	obj *infrav1alpha1.HCloudServer,
	s *hcloudclient.ServerInfo,
) (bool, error) {
	if obj.Spec.NetworkRef == nil || obj.Spec.NetworkRef.Name == "" {
		if obj.Status.AppliedNetworkID != 0 {
			if err := r.HCloudClient.DetachServerFromNetwork(ctx, s.ID, obj.Status.AppliedNetworkID); err != nil {
				return false, fmt.Errorf("detach server %d from previously managed network %d: %w", s.ID, obj.Status.AppliedNetworkID, err)
			}
			updated, err := r.HCloudClient.GetServer(ctx, s.ID)
			if err != nil {
				return false, fmt.Errorf("refresh server after network detach: %w", err)
			}
			if updated != nil {
				*s = *updated
			}
			obj.Status.AppliedNetworkID = 0
			r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, readyReasonNetworkDetached, "Detached server from previously managed private network")
			return true, nil
		}
		return false, nil
	}

	network := &infrav1alpha1.HCloudNetwork{}
	if err := r.Get(ctx, client.ObjectKey{Name: obj.Spec.NetworkRef.Name}, network); err != nil {
		return false, fmt.Errorf("get referenced HCloudNetwork %q: %w", obj.Spec.NetworkRef.Name, err)
	}
	if network.Status.NetworkID == 0 {
		return false, fmt.Errorf("referenced HCloudNetwork %q is not ready (status.networkID is empty)", network.Name)
	}

	migrated := false
	if obj.Status.AppliedNetworkID != 0 && obj.Status.AppliedNetworkID != network.Status.NetworkID {
		if err := r.HCloudClient.DetachServerFromNetwork(ctx, s.ID, obj.Status.AppliedNetworkID); err != nil {
			return false, fmt.Errorf(
				"detach server %d from previously managed network %d: %w",
				s.ID,
				obj.Status.AppliedNetworkID,
				err,
			)
		}
		updated, err := r.HCloudClient.GetServer(ctx, s.ID)
		if err != nil {
			return false, fmt.Errorf("refresh server after network migration detach: %w", err)
		}
		if updated != nil {
			*s = *updated
		}
		obj.Status.AppliedNetworkID = 0
		migrated = true
	}

	if !containsInt64(s.NetworkIDs, network.Status.NetworkID) {
		if err := r.HCloudClient.AttachServerToNetwork(ctx, s.ID, network.Status.NetworkID); err != nil {
			return false, fmt.Errorf(
				"attach server %d to network %q (%d): %w",
				s.ID,
				network.Name,
				network.Status.NetworkID,
				err,
			)
		}
		updated, err := r.HCloudClient.GetServer(ctx, s.ID)
		if err != nil {
			return false, fmt.Errorf("refresh server after network attach: %w", err)
		}
		if updated != nil {
			*s = *updated
		}
		if migrated {
			r.setCondition(
				obj,
				conditionTypeReady,
				metav1.ConditionTrue,
				readyReasonNetworkMigrated,
				fmt.Sprintf("Migrated server private network attachment to %q", network.Name),
			)
		} else {
			r.setCondition(
				obj,
				conditionTypeReady,
				metav1.ConditionTrue,
				readyReasonNetworkAttached,
				fmt.Sprintf("Attached server to private network %q", network.Name),
			)
		}
		obj.Status.AppliedNetworkID = network.Status.NetworkID
		return true, nil
	}
	obj.Status.AppliedNetworkID = network.Status.NetworkID
	return false, nil
}

func (r *HCloudServerReconciler) reconcileServerTypeMismatch(ctx context.Context, obj *infrav1alpha1.HCloudServer, s *hcloudclient.ServerInfo) (time.Duration, error) {
	desired := obj.Spec.ServerType

	switch s.State {
	case "initializing":
		r.applyServerStatus(obj, s)
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "Resizing", "Server is initializing; waiting before changing type")
		return resizeRequeueDelay, r.Status().Update(ctx, obj)

	case "running":
		if err := r.HCloudClient.PowerOffServer(ctx, s.ID); err != nil {
			return 0, err
		}
		s2, err := r.HCloudClient.GetServer(ctx, s.ID)
		if err != nil {
			return 0, fmt.Errorf("refresh server after power off: %w", err)
		}
		r.applyServerStatus(obj, s2)
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "Resizing", "Powered off server to change type")
		return resizeRequeueDelay, r.Status().Update(ctx, obj)

	case "stopping", "migrating", "rebuilding":
		r.applyServerStatus(obj, s)
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "Resizing", fmt.Sprintf("Waiting for server state %q to finish before changing type", s.State))
		return resizeRequeueDelay, r.Status().Update(ctx, obj)

	case "off":
		if err := r.HCloudClient.ChangeServerType(ctx, s.ID, desired, false); err != nil {
			return 0, err
		}
		s2, err := r.HCloudClient.GetServer(ctx, s.ID)
		if err != nil {
			return 0, fmt.Errorf("refresh server after change type: %w", err)
		}
		r.applyServerStatus(obj, s2)
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "Resizing", "Changed server type; will power on when disk and type have converged")
		return resizeRequeueDelay, r.Status().Update(ctx, obj)

	case "starting":
		r.applyServerStatus(obj, s)
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "Resizing", "Server is starting; waiting before continuing type change")
		return resizeRequeueDelay, r.Status().Update(ctx, obj)

	case "deleting":
		return 0, fmt.Errorf("server %d is deleting; cannot change type", s.ID)

	default:
		return 0, fmt.Errorf("unsupported server state %q for resize", s.State)
	}
}

func (r *HCloudServerReconciler) applyServerStatus(obj *infrav1alpha1.HCloudServer, s *hcloudclient.ServerInfo) {
	obj.Status.ServerID = s.ID
	obj.Status.State = s.State
	if s.PublicIPv4 != "" {
		obj.Status.PublicIPv4 = s.PublicIPv4
	}
	if s.PublicIPv6 != "" {
		obj.Status.PublicIPv6 = s.PublicIPv6
	}
}

// syncStatus copies live Hetzner server state into the CRD status and persists it.
func (r *HCloudServerReconciler) syncStatus(ctx context.Context, obj *infrav1alpha1.HCloudServer, s *hcloudclient.ServerInfo) error {
	r.applyServerStatus(obj, s)

	if s.ServerType == obj.Spec.ServerType && s.State == "running" {
		obj.Status.AppliedServerType = obj.Spec.ServerType
		if hasNetworkLifecycleReadyReason(obj) {
			return r.Status().Update(ctx, obj)
		}
		r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, "ServerRunning", "Hetzner server is running at desired type")
		return r.Status().Update(ctx, obj)
	}

	if s.State == "running" && s.ServerType != "" && s.ServerType != obj.Spec.ServerType {
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "ResizePending", "Server is running but type does not match spec")
		return r.Status().Update(ctx, obj)
	}

	r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "ServerNotReady", fmt.Sprintf("Hetzner server state is %q", s.State))
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

func containsInt64(items []int64, want int64) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func hasNetworkLifecycleReadyReason(obj *infrav1alpha1.HCloudServer) bool {
	cond := meta.FindStatusCondition(obj.Status.Conditions, conditionTypeReady)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return false
	}
	switch cond.Reason {
	case readyReasonNetworkAttached, readyReasonNetworkMigrated, readyReasonNetworkDetached:
		return true
	default:
		return false
	}
}
