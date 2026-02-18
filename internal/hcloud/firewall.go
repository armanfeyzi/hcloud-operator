package hcloud

import (
	"context"
	"fmt"
	"net"
	"slices"

	hcloudgo "github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// GetFirewall fetches a firewall by Hetzner ID.
func (c *Client) GetFirewall(ctx context.Context, id int64) (*FirewallInfo, error) {
	f, _, err := c.hc.Firewall.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetFirewall(%d): %w", id, err)
	}
	if f == nil {
		return nil, nil
	}
	return firewallInfoFromSDK(f), nil
}

// GetFirewallByName fetches a firewall by name.
func (c *Client) GetFirewallByName(ctx context.Context, name string) (*FirewallInfo, error) {
	f, _, err := c.hc.Firewall.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("hcloud: GetFirewallByName(%q): %w", name, err)
	}
	if f == nil {
		return nil, nil
	}
	return firewallInfoFromSDK(f), nil
}

// CreateFirewall creates a new firewall.
func (c *Client) CreateFirewall(ctx context.Context, opts FirewallCreateOpts) (*FirewallInfo, error) {
	rules, err := firewallRuleInfosToSDK(opts.Rules)
	if err != nil {
		return nil, err
	}
	applyTo, err := firewallApplyResourcesToSDK(opts.ApplyTo)
	if err != nil {
		return nil, err
	}
	result, _, err := c.hc.Firewall.Create(ctx, hcloudgo.FirewallCreateOpts{
		Name:    opts.Name,
		Labels:  opts.Labels,
		Rules:   rules,
		ApplyTo: applyTo,
	})
	if err != nil {
		return nil, fmt.Errorf("hcloud: CreateFirewall %q: %w", opts.Name, err)
	}
	return firewallInfoFromSDK(result.Firewall), nil
}

// DeleteFirewall deletes a firewall by ID (idempotent).
func (c *Client) DeleteFirewall(ctx context.Context, id int64) error {
	f, _, err := c.hc.Firewall.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetFirewall(%d): %w", id, err)
	}
	if f == nil {
		return nil
	}
	if _, err := c.hc.Firewall.Delete(ctx, f); err != nil {
		return fmt.Errorf("hcloud: DeleteFirewall(%d): %w", id, err)
	}
	return nil
}

// UpdateFirewallLabels updates labels on an existing firewall.
func (c *Client) UpdateFirewallLabels(ctx context.Context, id int64, labels map[string]string) error {
	f, _, err := c.hc.Firewall.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetFirewall(%d): %w", id, err)
	}
	if f == nil {
		return fmt.Errorf("hcloud: firewall %d not found", id)
	}
	if _, _, err := c.hc.Firewall.Update(ctx, f, hcloudgo.FirewallUpdateOpts{Labels: labels}); err != nil {
		return fmt.Errorf("hcloud: UpdateFirewallLabels(%d): %w", id, err)
	}
	return nil
}

// SetFirewallRules replaces all rules on the firewall.
func (c *Client) SetFirewallRules(ctx context.Context, id int64, rules []FirewallRuleInfo) error {
	f, _, err := c.hc.Firewall.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetFirewall(%d): %w", id, err)
	}
	if f == nil {
		return fmt.Errorf("hcloud: firewall %d not found", id)
	}
	sdkRules, err := firewallRuleInfosToSDK(rules)
	if err != nil {
		return err
	}
	if _, _, err := c.hc.Firewall.SetRules(ctx, f, hcloudgo.FirewallSetRulesOpts{Rules: sdkRules}); err != nil {
		return fmt.Errorf("hcloud: SetFirewallRules(%d): %w", id, err)
	}
	return nil
}

// ApplyFirewallResources applies the firewall to additional servers or label selectors.
func (c *Client) ApplyFirewallResources(ctx context.Context, id int64, resources []FirewallApplyResource) error {
	if len(resources) == 0 {
		return nil
	}
	f, _, err := c.hc.Firewall.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetFirewall(%d): %w", id, err)
	}
	if f == nil {
		return fmt.Errorf("hcloud: firewall %d not found", id)
	}
	sdk, err := firewallApplyResourcesToSDK(resources)
	if err != nil {
		return err
	}
	if _, _, err := c.hc.Firewall.ApplyResources(ctx, f, sdk); err != nil {
		return fmt.Errorf("hcloud: ApplyFirewallResources(%d): %w", id, err)
	}
	return nil
}

// RemoveFirewallResources removes the firewall from the given servers or label selectors.
func (c *Client) RemoveFirewallResources(ctx context.Context, id int64, resources []FirewallApplyResource) error {
	if len(resources) == 0 {
		return nil
	}
	f, _, err := c.hc.Firewall.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("hcloud: GetFirewall(%d): %w", id, err)
	}
	if f == nil {
		return fmt.Errorf("hcloud: firewall %d not found", id)
	}
	sdk, err := firewallApplyResourcesToSDK(resources)
	if err != nil {
		return err
	}
	if _, _, err := c.hc.Firewall.RemoveResources(ctx, f, sdk); err != nil {
		return fmt.Errorf("hcloud: RemoveFirewallResources(%d): %w", id, err)
	}
	return nil
}

func firewallInfoFromSDK(f *hcloudgo.Firewall) *FirewallInfo {
	if f == nil {
		return nil
	}
	info := &FirewallInfo{
		ID:   f.ID,
		Name: f.Name,
	}
	if f.Labels != nil {
		info.Labels = make(map[string]string, len(f.Labels))
		for k, v := range f.Labels {
			info.Labels[k] = v
		}
	}
	for _, r := range f.Rules {
		info.Rules = append(info.Rules, firewallRuleFromSDK(r))
	}
	for _, a := range f.AppliedTo {
		switch a.Type {
		case hcloudgo.FirewallResourceTypeServer:
			if a.Server != nil {
				info.AppliedTo = append(info.AppliedTo, FirewallApplyResource{
					Type:     "server",
					ServerID: a.Server.ID,
				})
			}
		case hcloudgo.FirewallResourceTypeLabelSelector:
			if a.LabelSelector != nil {
				info.AppliedTo = append(info.AppliedTo, FirewallApplyResource{
					Type:     "label_selector",
					Selector: a.LabelSelector.Selector,
				})
			}
		}
	}
	return info
}

func firewallRuleFromSDK(r hcloudgo.FirewallRule) FirewallRuleInfo {
	out := FirewallRuleInfo{
		Direction: string(r.Direction),
		Protocol:  string(r.Protocol),
		Port:      r.Port,
	}
	if r.Description != nil {
		d := *r.Description
		out.Description = &d
	}
	for _, ip := range r.SourceIPs {
		out.SourceIPs = append(out.SourceIPs, ip.String())
	}
	for _, ip := range r.DestinationIPs {
		out.DestinationIPs = append(out.DestinationIPs, ip.String())
	}
	slices.Sort(out.SourceIPs)
	slices.Sort(out.DestinationIPs)
	return out
}

func firewallRuleInfosToSDK(rules []FirewallRuleInfo) ([]hcloudgo.FirewallRule, error) {
	var out []hcloudgo.FirewallRule
	for i, r := range rules {
		rule := hcloudgo.FirewallRule{
			Direction: hcloudgo.FirewallRuleDirection(r.Direction),
			Protocol:  hcloudgo.FirewallRuleProtocol(r.Protocol),
			Port:      r.Port,
			Description: func() *string {
				if r.Description == nil {
					return nil
				}
				s := *r.Description
				return &s
			}(),
		}
		for _, s := range r.SourceIPs {
			_, ipnet, err := net.ParseCIDR(s)
			if err != nil {
				return nil, fmt.Errorf("hcloud: rule[%d] invalid source IP %q: %w", i, s, err)
			}
			rule.SourceIPs = append(rule.SourceIPs, *ipnet)
		}
		for _, s := range r.DestinationIPs {
			_, ipnet, err := net.ParseCIDR(s)
			if err != nil {
				return nil, fmt.Errorf("hcloud: rule[%d] invalid destination IP %q: %w", i, s, err)
			}
			rule.DestinationIPs = append(rule.DestinationIPs, *ipnet)
		}
		out = append(out, rule)
	}
	return out, nil
}

// Key returns a stable identity for diffing apply targets.
func (a FirewallApplyResource) Key() string {
	switch a.Type {
	case "server":
		return fmt.Sprintf("server:%d", a.ServerID)
	case "label_selector":
		return "label_selector:" + a.Selector
	default:
		return fmt.Sprintf("unknown:%s", a.Type)
	}
}

func firewallApplyResourcesToSDK(resources []FirewallApplyResource) ([]hcloudgo.FirewallResource, error) {
	var out []hcloudgo.FirewallResource
	for i, res := range resources {
		switch res.Type {
		case "server":
			if res.ServerID == 0 {
				return nil, fmt.Errorf("hcloud: apply[%d] server id is required", i)
			}
			out = append(out, hcloudgo.FirewallResource{
				Type:   hcloudgo.FirewallResourceTypeServer,
				Server: &hcloudgo.FirewallResourceServer{ID: res.ServerID},
			})
		case "label_selector":
			if res.Selector == "" {
				return nil, fmt.Errorf("hcloud: apply[%d] label selector is empty", i)
			}
			out = append(out, hcloudgo.FirewallResource{
				Type: hcloudgo.FirewallResourceTypeLabelSelector,
				LabelSelector: &hcloudgo.FirewallResourceLabelSelector{
					Selector: res.Selector,
				},
			})
		default:
			return nil, fmt.Errorf("hcloud: apply[%d] unknown type %q", i, res.Type)
		}
	}
	return out, nil
}
