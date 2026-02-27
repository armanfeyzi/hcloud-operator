package controller

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

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

const hcloudFirewallFinalizer = "infra.hkc.io/firewall-finalizer"

// HCloudFirewallReconciler reconciles HCloudFirewall objects.
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudfirewalls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudfirewalls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudfirewalls/finalizers,verbs=update
// +kubebuilder:rbac:groups=infra.hkc.io,resources=hcloudservers,verbs=get;list;watch
type HCloudFirewallReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	HCloudClient hcloudclient.Interface
}

func (r *HCloudFirewallReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.HCloudFirewall{}).
		Complete(r)
}

func (r *HCloudFirewallReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	fw := &infrav1alpha1.HCloudFirewall{}
	if err := r.Get(ctx, req.NamespacedName, fw); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !fw.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(fw, hcloudFirewallFinalizer) {
			log.Info("Handling firewall deletion", "firewallID", fw.Status.FirewallID)
			if err := r.deleteHCloudFirewall(ctx, fw); err != nil {
				return ctrl.Result{}, fmt.Errorf("delete Hetzner firewall: %w", err)
			}
			controllerutil.RemoveFinalizer(fw, hcloudFirewallFinalizer)
			if err := r.Update(ctx, fw); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(fw, hcloudFirewallFinalizer) {
		controllerutil.AddFinalizer(fw, hcloudFirewallFinalizer)
		if err := r.Update(ctx, fw); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.reconcileHCloudFirewall(ctx, fw); err != nil {
		r.setFirewallCondition(fw, conditionTypeReady, metav1.ConditionFalse, "ReconcileError", err.Error())
		_ = r.updateFirewallStatusWithRetry(ctx, fw)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

func (r *HCloudFirewallReconciler) reconcileHCloudFirewall(ctx context.Context, obj *infrav1alpha1.HCloudFirewall) error {
	desiredRules := firewallRulesFromSpec(obj.Spec.Rules)
	desiredApply, err := r.desiredApplyResources(ctx, obj)
	if err != nil {
		return err
	}

	var existing *hcloudclient.FirewallInfo
	if obj.Status.FirewallID != 0 {
		existing, err = r.HCloudClient.GetFirewall(ctx, obj.Status.FirewallID)
		if err != nil {
			return fmt.Errorf("fetch firewall by ID: %w", err)
		}
	}
	if existing == nil {
		existing, err = r.HCloudClient.GetFirewallByName(ctx, obj.Name)
		if err != nil {
			return fmt.Errorf("fetch firewall by name: %w", err)
		}
	}

	if existing == nil {
		created, err := r.HCloudClient.CreateFirewall(ctx, hcloudclient.FirewallCreateOpts{
			Name:    obj.Name,
			Labels:  obj.Spec.Labels,
			Rules:   desiredRules,
			ApplyTo: desiredApply,
		})
		if err != nil {
			return fmt.Errorf("create Hetzner firewall: %w", err)
		}
		obj.Status.FirewallID = created.ID
		r.setFirewallCondition(obj, conditionTypeReady, metav1.ConditionTrue, "FirewallCreated", "Firewall created in Hetzner Cloud")
		return r.updateFirewallStatusWithRetry(ctx, obj)
	}

	obj.Status.FirewallID = existing.ID

	if !labelsMatch(obj.Spec.Labels, existing.Labels) {
		if err := r.HCloudClient.UpdateFirewallLabels(ctx, existing.ID, normalizeLabelsMap(obj.Spec.Labels)); err != nil {
			return fmt.Errorf("update firewall labels: %w", err)
		}
		refreshed, err := r.HCloudClient.GetFirewall(ctx, existing.ID)
		if err != nil {
			return fmt.Errorf("refresh firewall after label update: %w", err)
		}
		if refreshed != nil {
			existing = refreshed
		}
	}

	if !firewallRulesMatch(desiredRules, existing.Rules) {
		if err := r.HCloudClient.SetFirewallRules(ctx, existing.ID, desiredRules); err != nil {
			return fmt.Errorf("set firewall rules: %w", err)
		}
		refreshed, err := r.HCloudClient.GetFirewall(ctx, existing.ID)
		if err != nil {
			return fmt.Errorf("refresh firewall after set rules: %w", err)
		}
		if refreshed != nil {
			existing = refreshed
		}
	}

	remove, add := partitionApplyTargets(existing.AppliedTo, desiredApply)
	if len(remove) > 0 {
		if err := r.HCloudClient.RemoveFirewallResources(ctx, existing.ID, remove); err != nil {
			return fmt.Errorf("remove firewall resources: %w", err)
		}
	}
	if len(add) > 0 {
		if err := r.HCloudClient.ApplyFirewallResources(ctx, existing.ID, add); err != nil {
			return fmt.Errorf("apply firewall resources: %w", err)
		}
	}

	r.setFirewallCondition(obj, conditionTypeReady, metav1.ConditionTrue, "FirewallReady", "Firewall rules and attachments are in sync")
	return r.updateFirewallStatusWithRetry(ctx, obj)
}

func (r *HCloudFirewallReconciler) desiredApplyResources(ctx context.Context, fw *infrav1alpha1.HCloudFirewall) ([]hcloudclient.FirewallApplyResource, error) {
	if fw.Spec.ApplyTo == nil {
		return nil, nil
	}
	var out []hcloudclient.FirewallApplyResource
	if sel := fw.Spec.ApplyTo.LabelSelector; sel != "" {
		out = append(out, hcloudclient.FirewallApplyResource{
			Type:     "label_selector",
			Selector: sel,
		})
	}
	seenServers := make(map[int64]struct{})
	for _, ref := range fw.Spec.ApplyTo.ServerRefs {
		if ref.Name == "" {
			continue
		}
		srv := &infrav1alpha1.HCloudServer{}
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name}, srv); err != nil {
			return nil, fmt.Errorf("resolve server ref %q: %w", ref.Name, err)
		}
		if srv.Status.ServerID == 0 {
			continue
		}
		if _, dup := seenServers[srv.Status.ServerID]; dup {
			continue
		}
		seenServers[srv.Status.ServerID] = struct{}{}
		out = append(out, hcloudclient.FirewallApplyResource{
			Type:     "server",
			ServerID: srv.Status.ServerID,
		})
	}
	return out, nil
}

func (r *HCloudFirewallReconciler) deleteHCloudFirewall(ctx context.Context, obj *infrav1alpha1.HCloudFirewall) error {
	if obj.Status.FirewallID == 0 {
		return nil
	}
	return r.HCloudClient.DeleteFirewall(ctx, obj.Status.FirewallID)
}

func (r *HCloudFirewallReconciler) setFirewallCondition(
	obj *infrav1alpha1.HCloudFirewall,
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

func firewallRulesFromSpec(rules []infrav1alpha1.HCloudFirewallRule) []hcloudclient.FirewallRuleInfo {
	out := make([]hcloudclient.FirewallRuleInfo, 0, len(rules))
	for _, r := range rules {
		info := hcloudclient.FirewallRuleInfo{
			Direction:      r.Direction,
			Protocol:       r.Protocol,
			Port:           copyStrPtr(r.Port),
			Description:    copyStrPtr(r.Description),
			SourceIPs:      append([]string{}, r.SourceIPs...),
			DestinationIPs: append([]string{}, r.DestinationIPs...),
		}
		slices.Sort(info.SourceIPs)
		slices.Sort(info.DestinationIPs)
		out = append(out, info)
	}
	return out
}

func copyStrPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

func normalizeLabelsMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return maps.Clone(m)
}

func labelsMatch(spec, cloud map[string]string) bool {
	a := normalizeLabelsMap(spec)
	b := normalizeLabelsMap(cloud)
	return maps.Equal(a, b)
}

func firewallRulesMatch(desired, observed []hcloudclient.FirewallRuleInfo) bool {
	if len(desired) != len(observed) {
		return false
	}
	counts := make(map[string]int)
	for _, r := range desired {
		counts[canonicalFirewallRule(r)]++
	}
	for _, r := range observed {
		k := canonicalFirewallRule(r)
		counts[k]--
		if counts[k] < 0 {
			return false
		}
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

func canonicalFirewallRule(r hcloudclient.FirewallRuleInfo) string {
	sip := slices.Clone(r.SourceIPs)
	slices.Sort(sip)
	dip := slices.Clone(r.DestinationIPs)
	slices.Sort(dip)
	port := ""
	if r.Port != nil {
		port = *r.Port
	}
	desc := ""
	if r.Description != nil {
		desc = *r.Description
	}
	return strings.Join([]string{
		r.Direction, r.Protocol, port, desc,
		strings.Join(sip, ","),
		strings.Join(dip, ","),
	}, "\x00")
}

func partitionApplyTargets(current, desired []hcloudclient.FirewallApplyResource) (remove, add []hcloudclient.FirewallApplyResource) {
	cur := make(map[string]hcloudclient.FirewallApplyResource, len(current))
	for _, c := range current {
		cur[c.Key()] = c
	}
	des := make(map[string]hcloudclient.FirewallApplyResource, len(desired))
	for _, d := range desired {
		des[d.Key()] = d
	}
	for k, c := range cur {
		if _, ok := des[k]; !ok {
			remove = append(remove, c)
		}
	}
	for k, d := range des {
		if _, ok := cur[k]; !ok {
			add = append(add, d)
		}
	}
	return remove, add
}

func (r *HCloudFirewallReconciler) updateFirewallStatusWithRetry(ctx context.Context, obj *infrav1alpha1.HCloudFirewall) error {
	key := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
	desiredStatus := obj.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &infrav1alpha1.HCloudFirewall{}
		if err := r.Get(ctx, key, current); err != nil {
			return err
		}
		current.Status = *desiredStatus.DeepCopy()
		return r.Status().Update(ctx, current)
	})
}
