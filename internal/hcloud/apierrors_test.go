package hcloud

import (
	"fmt"
	"testing"

	hcloudgo "github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func TestIsResourceInUse_wrapped(t *testing.T) {
	err := fmt.Errorf("delete: %w", hcloudgo.Error{Code: hcloudgo.ErrorCodeResourceInUse, Message: "firewall is still in use"})
	if !IsResourceInUse(err) {
		t.Fatal("expected resource in use")
	}
}

func TestIsFloatingIPNotAssigned_wrapped(t *testing.T) {
	err := fmt.Errorf("hcloud: UnassignFloatingIP(1): %w", hcloudgo.Error{
		Code:    hcloudgo.ErrorCodeInvalidInput,
		Message: "not assigned to any resource",
	})
	if !IsFloatingIPNotAssigned(err) {
		t.Fatal("expected not assigned")
	}
}

func TestIsFirewallAlreadyRemoved(t *testing.T) {
	err := hcloudgo.Error{Code: hcloudgo.ErrorCodeFirewallAlreadyRemoved, Message: "already removed"}
	if !IsFirewallAlreadyRemoved(err) {
		t.Fatal("expected firewall already removed")
	}
}
