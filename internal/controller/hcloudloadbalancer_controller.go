package controller

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
)

const hcloudLoadBalancerFinalizer = "infra.hkc.io/loadbalancer-finalizer"

// HCloudLoadBalancerReconciler reconciles HCloudLoadBalancer objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudloadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudloadbalancers/finalizers,verbs=update
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudcertificates,verbs=get;list;watch
type HCloudLoadBalancerReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudLoadBalancer{}).
		Watches(
			&infrav1alpha1.HCloudServer{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllLoadBalancersForServerChange),
		).
		Watches(
			&infrav1alpha1.HCloudCertificate{},
			handler.EnqueueRequestsFromMapFunc(r.mapCertificateToLoadBalancers),
		).
		Complete(r)
}

func (r *HCloudLoadBalancerReconciler) mapCertificateToLoadBalancers(ctx context.Context, obj client.Object) []reconcile.Request {
	cert, ok := obj.(*infrav1alpha1.HCloudCertificate)
	if !ok {
		return nil
	}

	var lbs infrav1alpha1.HCloudLoadBalancerList
	if err := r.List(ctx, &lbs); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0)
	for i := range lbs.Items {
		for _, svc := range lbs.Items[i].Spec.Services {
			for _, ref := range svc.CertificateRefs {
				if ref.Name == cert.Name {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: lbs.Items[i].Name}})
					break
				}
			}
		}
	}
	return requests
}

// enqueueAllLoadBalancersForServerChange requeues every load balancer when any server changes
// (labels, status ServerID, etc.) so serverSelector attachment stays in sync without waiting for LB periodic requeue.
func (r *HCloudLoadBalancerReconciler) enqueueAllLoadBalancersForServerChange(ctx context.Context, _ client.Object) []reconcile.Request {
	var list infrav1alpha1.HCloudLoadBalancerList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: list.Items[i].Name}})
	}
	return reqs
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
		_ = r.updateLoadBalancerStatusWithRetry(ctx, lb)
		return ctrl.Result{}, err
	}

	if err := r.reconcileLoadBalancer(ctx, lb, selectedServerIDs); err != nil {
		r.setCondition(lb, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.updateLoadBalancerStatusWithRetry(ctx, lb)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudLoadBalancerReconciler) reconcileLoadBalancer(
	ctx context.Context,
	obj *infrav1alpha1.HCloudLoadBalancer,
	selectedServerIDs []int64,
) error {
	desiredServices, pending, err := r.loadBalancerServicesFromSpec(ctx, obj.Spec.Services)
	if err != nil {
		return err
	}
	if pending {
		r.setCondition(obj, conditionTypeReady, metav1.ConditionFalse, "CertificatePending", "Waiting for referenced HCloudCertificate resources")
		return r.updateLoadBalancerStatusWithRetry(ctx, obj)
	}

	return r.reconcileLoadBalancerWithServices(ctx, obj, selectedServerIDs, desiredServices)
}

func (r *HCloudLoadBalancerReconciler) reconcileLoadBalancerWithServices(
	ctx context.Context,
	obj *infrav1alpha1.HCloudLoadBalancer,
	selectedServerIDs []int64,
	desiredServices []hcloudclient.LoadBalancerServiceInfo,
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

	if err := r.HCloudClient.SyncLoadBalancerServices(ctx, existing.ID, desiredServices); err != nil {
		return fmt.Errorf("sync load balancer services: %w", err)
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
	return r.updateLoadBalancerStatusWithRetry(ctx, obj)
}

func (r *HCloudLoadBalancerReconciler) loadBalancerServicesFromSpec(
	ctx context.Context,
	specs []infrav1alpha1.HCloudLoadBalancerServiceSpec,
) ([]hcloudclient.LoadBalancerServiceInfo, bool, error) {
	if len(specs) == 0 {
		return nil, false, nil
	}
	out := make([]hcloudclient.LoadBalancerServiceInfo, 0, len(specs))
	pending := false
	for _, spec := range specs {
		svc := hcloudclient.LoadBalancerServiceInfo{
			Protocol:        spec.Protocol,
			ListenPort:      int(spec.ListenPort),
			DestinationPort: int(spec.DestinationPort),
		}
		if spec.Proxyprotocol != nil {
			svc.Proxyprotocol = *spec.Proxyprotocol
		}
		if len(spec.CertificateRefs) > 0 {
			ids, certPending, err := r.resolveCertificateIDs(ctx, spec.CertificateRefs)
			if err != nil {
				return nil, false, err
			}
			if certPending {
				pending = true
			}
			svc.CertificateIDs = ids
		}
		if spec.HealthCheck != nil {
			hc := hcloudclient.LoadBalancerHealthCheckInfo{
				Protocol: spec.HealthCheck.Protocol,
			}
			if spec.HealthCheck.Port != nil {
				port := int(*spec.HealthCheck.Port)
				hc.Port = &port
			}
			if spec.HealthCheck.IntervalSeconds != nil {
				interval := time.Duration(*spec.HealthCheck.IntervalSeconds) * time.Second
				hc.Interval = &interval
			}
			if spec.HealthCheck.TimeoutSeconds != nil {
				timeout := time.Duration(*spec.HealthCheck.TimeoutSeconds) * time.Second
				hc.Timeout = &timeout
			}
			if spec.HealthCheck.Retries != nil {
				retries := int(*spec.HealthCheck.Retries)
				hc.Retries = &retries
			}
			if spec.HealthCheck.HTTP != nil {
				hc.HTTP = &hcloudclient.LoadBalancerHealthCheckHTTPInfo{
					Domain:      spec.HealthCheck.HTTP.Domain,
					Path:        spec.HealthCheck.HTTP.Path,
					Response:    spec.HealthCheck.HTTP.Response,
					StatusCodes: append([]string{}, spec.HealthCheck.HTTP.StatusCodes...),
					TLS:         spec.HealthCheck.HTTP.TLS,
				}
			}
			svc.HealthCheck = &hc
		}
		out = append(out, svc)
	}
	return out, pending, nil
}

func (r *HCloudLoadBalancerReconciler) resolveCertificateIDs(
	ctx context.Context,
	refs []corev1.LocalObjectReference,
) ([]int64, bool, error) {
	ids := make([]int64, 0, len(refs))
	pending := false
	for _, ref := range refs {
		if ref.Name == "" {
			continue
		}
		cert := &infrav1alpha1.HCloudCertificate{}
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name}, cert); err != nil {
			if apierrors.IsNotFound(err) {
				pending = true
				continue
			}
			return nil, false, fmt.Errorf("get referenced HCloudCertificate %q: %w", ref.Name, err)
		}
		if cert.Status.CertificateID == 0 {
			pending = true
			continue
		}
		info, err := r.HCloudClient.GetCertificate(ctx, cert.Status.CertificateID)
		if err != nil {
			return nil, false, fmt.Errorf("fetch Hetzner certificate %d: %w", cert.Status.CertificateID, err)
		}
		if info == nil || !hcloudclient.CertificateReady(info) {
			pending = true
			continue
		}
		ids = append(ids, cert.Status.CertificateID)
	}
	return ids, pending, nil
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

func (r *HCloudLoadBalancerReconciler) updateLoadBalancerStatusWithRetry(ctx context.Context, obj *infrav1alpha1.HCloudLoadBalancer) error {
	key := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
	desiredStatus := obj.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &infrav1alpha1.HCloudLoadBalancer{}
		if err := r.Get(ctx, key, current); err != nil {
			return err
		}
		current.Status = *desiredStatus.DeepCopy()
		return r.Status().Update(ctx, current)
	})
}
