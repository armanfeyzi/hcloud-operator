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

const hcloudCertificateFinalizer = "infra.hkc.io/certificate-finalizer"

// HCloudCertificateReconciler reconciles HCloudCertificate objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudcertificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudcertificates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudcertificates/finalizers,verbs=update
type HCloudCertificateReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudCertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudCertificate{}).
		Complete(r)
}

func (r *HCloudCertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cert := &infrav1alpha1.HCloudCertificate{}
	if err := r.Get(ctx, req.NamespacedName, cert); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cert.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(cert, hcloudCertificateFinalizer) {
			log.Info("Handling certificate deletion", "certificateID", cert.Status.CertificateID)
			if err := r.deleteHCloudCertificate(ctx, cert); err != nil {
				return ctrl.Result{}, fmt.Errorf("delete Hetzner certificate: %w", err)
			}
			controllerutil.RemoveFinalizer(cert, hcloudCertificateFinalizer)
			if err := r.Update(ctx, cert); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(cert, hcloudCertificateFinalizer) {
		controllerutil.AddFinalizer(cert, hcloudCertificateFinalizer)
		if err := r.Update(ctx, cert); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	pending, err := r.reconcileHCloudCertificate(ctx, cert)
	if err != nil {
		r.setCertificateCondition(cert, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.updateCertificateStatusWithRetry(ctx, cert)
		return ctrl.Result{}, err
	}
	if pending {
		r.setCertificateCondition(cert, conditionTypeReady, metav1.ConditionFalse, "CertificatePending", "Waiting for managed certificate issuance")
		_ = r.updateCertificateStatusWithRetry(ctx, cert)
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudCertificateReconciler) reconcileHCloudCertificate(ctx context.Context, obj *infrav1alpha1.HCloudCertificate) (bool, error) {
	log := log.FromContext(ctx)

	var existing *hcloudclient.CertificateInfo
	var err error

	if obj.Status.CertificateID != 0 {
		existing, err = r.HCloudClient.GetCertificate(ctx, obj.Status.CertificateID)
		if err != nil {
			return false, fmt.Errorf("fetch certificate by ID: %w", err)
		}
	}
	if existing == nil {
		existing, err = r.HCloudClient.GetCertificateByName(ctx, obj.Name)
		if err != nil {
			return false, fmt.Errorf("fetch certificate by name: %w", err)
		}
	}

	if existing == nil {
		log.Info("Creating new Hetzner certificate", "name", obj.Name, "type", obj.Spec.Type)
		created, err := r.HCloudClient.CreateCertificate(ctx, hcloudclient.CertificateCreateOpts{
			Name:        obj.Name,
			Type:        obj.Spec.Type,
			Labels:      obj.Spec.Labels,
			Certificate: obj.Spec.Certificate,
			PrivateKey:  obj.Spec.PrivateKey,
			DomainNames: obj.Spec.DomainNames,
		})
		if err != nil {
			return false, fmt.Errorf("create Hetzner certificate: %w", err)
		}
		existing = created
	}

	if err := r.ensureCertificateMetadata(ctx, obj, existing); err != nil {
		return false, err
	}

	refreshed, err := r.HCloudClient.GetCertificate(ctx, existing.ID)
	if err != nil {
		return false, fmt.Errorf("refresh certificate: %w", err)
	}
	if refreshed == nil {
		return false, fmt.Errorf("certificate %d disappeared after metadata sync", existing.ID)
	}

	r.syncCertificateStatus(obj, refreshed)
	if !hcloudclient.CertificateReady(refreshed) {
		return true, nil
	}

	r.setCertificateCondition(obj, conditionTypeReady, metav1.ConditionTrue, "CertificateReady", "Certificate is provisioned")
	if err := r.updateCertificateStatusWithRetry(ctx, obj); err != nil {
		return false, err
	}
	return false, nil
}

func (r *HCloudCertificateReconciler) ensureCertificateMetadata(ctx context.Context, obj *infrav1alpha1.HCloudCertificate, existing *hcloudclient.CertificateInfo) error {
	if mapsEqual(existing.Labels, obj.Spec.Labels) {
		return nil
	}
	if err := r.HCloudClient.UpdateCertificate(ctx, existing.ID, hcloudclient.CertificateUpdateOpts{
		Labels: cloneStringMapController(obj.Spec.Labels),
	}); err != nil {
		return fmt.Errorf("update certificate metadata: %w", err)
	}
	return nil
}

func (r *HCloudCertificateReconciler) syncCertificateStatus(obj *infrav1alpha1.HCloudCertificate, existing *hcloudclient.CertificateInfo) {
	obj.Status.CertificateID = existing.ID
	obj.Status.DomainNames = append([]string{}, existing.DomainNames...)
	obj.Status.Fingerprint = existing.Fingerprint
	obj.Status.IssuanceStatus = existing.IssuanceStatus
	if !existing.NotValidBefore.IsZero() {
		t := metav1.NewTime(existing.NotValidBefore)
		obj.Status.NotValidBefore = &t
	}
	if !existing.NotValidAfter.IsZero() {
		t := metav1.NewTime(existing.NotValidAfter)
		obj.Status.NotValidAfter = &t
	}
}

func (r *HCloudCertificateReconciler) deleteHCloudCertificate(ctx context.Context, obj *infrav1alpha1.HCloudCertificate) error {
	if obj.Status.CertificateID == 0 {
		return nil
	}
	return r.HCloudClient.DeleteCertificate(ctx, obj.Status.CertificateID)
}

func (r *HCloudCertificateReconciler) setCertificateCondition(
	obj *infrav1alpha1.HCloudCertificate,
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

func (r *HCloudCertificateReconciler) updateCertificateStatusWithRetry(ctx context.Context, obj *infrav1alpha1.HCloudCertificate) error {
	key := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
	desiredStatus := obj.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &infrav1alpha1.HCloudCertificate{}
		if err := r.Get(ctx, key, current); err != nil {
			return err
		}
		current.Status = *desiredStatus.DeepCopy()
		return r.Status().Update(ctx, current)
	})
}
