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

const hcloudVolumeFinalizer = "infra.hkc.io/volume-finalizer"
const hcloudVolumeByServerRefNameField = "spec.serverRef.name"

// HCloudVolumeReconciler reconciles HCloudVolume objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudvolumes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudvolumes/finalizers,verbs=update
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch
type HCloudVolumeReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &infrav1alpha1.HCloudVolume{}, hcloudVolumeByServerRefNameField, func(rawObj client.Object) []string {
		vol, ok := rawObj.(*infrav1alpha1.HCloudVolume)
		if !ok || vol.Spec.ServerRef == nil || vol.Spec.ServerRef.Name == "" {
			return nil
		}
		return []string{vol.Spec.ServerRef.Name}
	}); err != nil {
		return fmt.Errorf("index HCloudVolume by serverRef name: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudVolume{}).
		Watches(
			&infrav1alpha1.HCloudServer{},
			handler.EnqueueRequestsFromMapFunc(r.mapServerToVolumes),
		).
		Complete(r)
}

func (r *HCloudVolumeReconciler) mapServerToVolumes(ctx context.Context, obj client.Object) []reconcile.Request {
	server, ok := obj.(*infrav1alpha1.HCloudServer)
	if !ok {
		return nil
	}

	var volumes infrav1alpha1.HCloudVolumeList
	if err := r.List(ctx, &volumes, client.MatchingFields{hcloudVolumeByServerRefNameField: server.Name}); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(volumes.Items))
	for i := range volumes.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: volumes.Items[i].Namespace,
				Name:      volumes.Items[i].Name,
			},
		})
	}
	return requests
}

func (r *HCloudVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	volume := &infrav1alpha1.HCloudVolume{}
	if err := r.Get(ctx, req.NamespacedName, volume); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// ── 1. Handle Deletion ───────────────────────────────────────────────────
	if !volume.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(volume, hcloudVolumeFinalizer) {
			log.Info("Handling deletion", "volumeID", volume.Status.VolumeID)
			if volume.Status.VolumeID != 0 {
				if err := r.HCloudClient.DeleteVolume(ctx, volume.Status.VolumeID); err != nil {
					return ctrl.Result{}, fmt.Errorf("delete volume: %w", err)
				}
			}
			controllerutil.RemoveFinalizer(volume, hcloudVolumeFinalizer)
			if err := r.Update(ctx, volume); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// ── 2. Ensure finalizer ──────────────────────────────────────────────────
	if !controllerutil.ContainsFinalizer(volume, hcloudVolumeFinalizer) {
		controllerutil.AddFinalizer(volume, hcloudVolumeFinalizer)
		if err := r.Update(ctx, volume); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// ── 3. Resolve Target Server ─────────────────────────────────────────────
	var targetServerID int64
	if volume.Spec.ServerRef != nil {
		serverObj := &infrav1alpha1.HCloudServer{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: volume.Namespace,
			Name:      volume.Spec.ServerRef.Name,
		}, serverObj)

		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Waiting for target server to be created", "server", volume.Spec.ServerRef.Name)
				r.setCondition(volume, conditionTypeReady, metav1.ConditionFalse, "ServerPending", "Target server not found")
				_ = r.updateVolumeStatusWithRetry(ctx, volume)
				return ctrl.Result{RequeueAfter: requeueDelay}, nil
			}
			return ctrl.Result{}, fmt.Errorf("get target server: %w", err)
		}

		if serverObj.Status.ServerID == 0 {
			log.Info("Waiting for target server to be provisioned", "server", volume.Spec.ServerRef.Name)
			r.setCondition(volume, conditionTypeReady, metav1.ConditionFalse, "ServerPending", "Target server not yet provisioned in Hetzner")
			_ = r.updateVolumeStatusWithRetry(ctx, volume)
			return ctrl.Result{RequeueAfter: requeueDelay}, nil
		}

		targetServerID = serverObj.Status.ServerID
	}

	// ── 4. Reconcile Hetzner Volume ──────────────────────────────────────────
	if err := r.reconcileHCloudVolume(ctx, volume, targetServerID); err != nil {
		r.setCondition(volume, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.updateVolumeStatusWithRetry(ctx, volume)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudVolumeReconciler) reconcileHCloudVolume(ctx context.Context, obj *infrav1alpha1.HCloudVolume, targetServerID int64) error {
	log := log.FromContext(ctx)

	// Fetch existing volume by ID or Name
	var existing *hcloudclient.VolumeInfo
	var err error

	if obj.Status.VolumeID != 0 {
		existing, err = r.HCloudClient.GetVolume(ctx, obj.Status.VolumeID)
		if err != nil {
			return fmt.Errorf("fetch volume by ID: %w", err)
		}
	}

	if existing == nil {
		existing, err = r.HCloudClient.GetVolumeByName(ctx, obj.Name)
		if err != nil {
			return fmt.Errorf("fetch volume by name: %w", err)
		}
	}

	// Create if not exists
	if existing == nil {
		log.Info("Creating new Hetzner volume", "name", obj.Name, "size", obj.Spec.Size)
		opts := hcloudclient.VolumeCreateOpts{
			Name:      obj.Name,
			Size:      obj.Spec.Size,
			ServerID:  targetServerID,
			Location:  obj.Spec.Location,
			Format:    obj.Spec.Format,
			Automount: true,
			Labels:    obj.Spec.Labels,
		}

		created, err := r.HCloudClient.CreateVolume(ctx, opts)
		if err != nil {
			return fmt.Errorf("create volume: %w", err)
		}
		existing = created
	}

	// Attach if needed
	if existing.ServerID != targetServerID {
		if existing.ServerID != 0 {
			log.Info("Detaching volume from old server", "volumeID", existing.ID, "oldServerID", existing.ServerID)
			if err := r.HCloudClient.DetachVolume(ctx, existing.ID); err != nil {
				return fmt.Errorf("detach volume: %w", err)
			}
		}

		if targetServerID != 0 {
			log.Info("Attaching volume to server", "volumeID", existing.ID, "serverID", targetServerID)
			if err := r.HCloudClient.AttachVolume(ctx, existing.ID, targetServerID); err != nil {
				return fmt.Errorf("attach volume: %w", err)
			}
			existing.ServerID = targetServerID
		}
	}

	// Sync status
	obj.Status.VolumeID = existing.ID
	obj.Status.State = existing.State
	obj.Status.AttachedServerID = existing.ServerID
	if existing.LinuxDevice != "" {
		obj.Status.LinuxDevice = existing.LinuxDevice
	}
	r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, "VolumeReady", "Volume is provisioned and attached")

	return r.updateVolumeStatusWithRetry(ctx, obj)
}

func (r *HCloudVolumeReconciler) setCondition(
	obj *infrav1alpha1.HCloudVolume,
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

func (r *HCloudVolumeReconciler) updateVolumeStatusWithRetry(ctx context.Context, obj *infrav1alpha1.HCloudVolume) error {
	key := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
	desiredStatus := obj.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &infrav1alpha1.HCloudVolume{}
		if err := r.Get(ctx, key, current); err != nil {
			return err
		}
		current.Status = *desiredStatus.DeepCopy()
		return r.Status().Update(ctx, current)
	})
}
