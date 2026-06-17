package controller

import (
	"context"
	"fmt"

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
	base := &basereconcile.BaseReconciler[*infrav1alpha1.HCloudVolume]{
		Client:   r.Client,
		Recorder: mgr.GetEventRecorderFor("hcloud-volume"),
		Resource: r,
	}

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
		Complete(base)
}

func (r *HCloudVolumeReconciler) NewObject() *infrav1alpha1.HCloudVolume {
	return &infrav1alpha1.HCloudVolume{}
}

func (r *HCloudVolumeReconciler) FinalizerName() string { return hcloudVolumeFinalizer }

func (r *HCloudVolumeReconciler) Kind() string { return "HCloudVolume" }

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

func (r *HCloudVolumeReconciler) Reconcile(ctx context.Context, volume *infrav1alpha1.HCloudVolume) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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
				return ctrl.Result{RequeueAfter: requeueDelay}, nil
			}
			return ctrl.Result{}, fmt.Errorf("get target server: %w", err)
		}

		if serverObj.Status.ServerID == 0 {
			log.Info("Waiting for target server to be provisioned", "server", volume.Spec.ServerRef.Name)
			r.setCondition(volume, conditionTypeReady, metav1.ConditionFalse, "ServerPending", "Target server not yet provisioned in Hetzner")
			return ctrl.Result{RequeueAfter: requeueDelay}, nil
		}

		targetServerID = serverObj.Status.ServerID
	}

	if err := r.reconcileHCloudVolume(ctx, volume, targetServerID); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudVolumeReconciler) Delete(ctx context.Context, volume *infrav1alpha1.HCloudVolume) error {
	if volume.Status.VolumeID == 0 {
		return nil
	}
	if err := r.detachVolumeIfAttached(ctx, volume); err != nil {
		return err
	}
	return r.HCloudClient.DeleteVolume(ctx, volume.Status.VolumeID)
}

func (r *HCloudVolumeReconciler) detachVolumeIfAttached(ctx context.Context, volume *infrav1alpha1.HCloudVolume) error {
	log := log.FromContext(ctx)

	attachedServerID := volume.Status.AttachedServerID
	existing, err := r.HCloudClient.GetVolume(ctx, volume.Status.VolumeID)
	if err != nil {
		return fmt.Errorf("fetch volume for deletion: %w", err)
	}
	if existing != nil && existing.ServerID != 0 {
		attachedServerID = existing.ServerID
	}

	if attachedServerID == 0 {
		return nil
	}

	log.Info("Detaching volume before delete", "volumeID", volume.Status.VolumeID, "serverID", attachedServerID)
	if err := r.HCloudClient.DetachVolume(ctx, volume.Status.VolumeID); err != nil {
		return fmt.Errorf("detach volume before delete: %w", err)
	}
	return nil
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
			Automount: targetServerID != 0, // Hetzner requires a server when automount is true
			Labels:    obj.Spec.Labels,
		}

		created, err := r.HCloudClient.CreateVolume(ctx, opts)
		if err != nil {
			return fmt.Errorf("create volume: %w", err)
		}
		existing = created
	}

	if existing.Size < obj.Spec.Size {
		log.Info("Resizing Hetzner volume", "volumeID", existing.ID, "from", existing.Size, "to", obj.Spec.Size)
		if err := r.HCloudClient.ResizeVolume(ctx, existing.ID, obj.Spec.Size); err != nil {
			return fmt.Errorf("resize volume: %w", err)
		}
		refreshed, err := r.HCloudClient.GetVolume(ctx, existing.ID)
		if err != nil {
			return fmt.Errorf("refresh volume after resize: %w", err)
		}
		if refreshed == nil {
			return fmt.Errorf("volume %d disappeared after resize", existing.ID)
		}
		existing = refreshed
	} else if existing.Size > obj.Spec.Size {
		return fmt.Errorf("volume size %d GB in Hetzner exceeds spec.size %d GB; shrink is not supported", existing.Size, obj.Spec.Size)
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
	obj.Status.AppliedSize = existing.Size
	if existing.LinuxDevice != "" {
		obj.Status.LinuxDevice = existing.LinuxDevice
	}
	r.setCondition(obj, conditionTypeReady, metav1.ConditionTrue, "VolumeReady", "Volume is provisioned and attached")

	return nil
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
