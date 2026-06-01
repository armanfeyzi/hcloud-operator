package hcloud

import (
	"context"
	"testing"
)

func TestDeleteFirewall_detachesAppliedResourcesFirst(t *testing.T) {
	fake := NewFakeClient()
	ctx := context.Background()

	created, err := fake.CreateFirewall(ctx, FirewallCreateOpts{
		Name: "edge",
		Rules: []FirewallRuleInfo{{
			Direction: "in",
			Protocol:  "tcp",
			Port:      strPtr("22"),
			SourceIPs: []string{"0.0.0.0/0"},
		}},
		ApplyTo: []FirewallApplyResource{{
			Type:     "server",
			ServerID: 42,
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.AppliedTo) != 1 {
		t.Fatalf("expected applied server, got %+v", created.AppliedTo)
	}

	if err := fake.DeleteFirewall(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := fake.GetFirewall(ctx, created.ID); err != nil {
		t.Fatalf("get after delete: %v", err)
	}
}

func strPtr(s string) *string { return &s }
