package hcloud

import (
	"errors"
	"strings"

	hcloudgo "github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// AsAPIError returns a Hetzner Cloud API error if err wraps one.
func AsAPIError(err error) (hcloudgo.Error, bool) {
	var apiErr hcloudgo.Error
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return apiErr, false
}

// IsResourceInUse reports whether err is a firewall (or similar) "resource in use" error.
func IsResourceInUse(err error) bool {
	apiErr, ok := AsAPIError(err)
	return ok && apiErr.Code == hcloudgo.ErrorCodeResourceInUse
}

// IsFirewallAlreadyRemoved reports whether a firewall detach targeted a resource that no longer has it.
func IsFirewallAlreadyRemoved(err error) bool {
	apiErr, ok := AsAPIError(err)
	return ok && apiErr.Code == hcloudgo.ErrorCodeFirewallAlreadyRemoved
}

// IsFloatingIPNotAssigned reports whether unassign was called when the floating IP has no server.
func IsFloatingIPNotAssigned(err error) bool {
	if apiErr, ok := AsAPIError(err); ok {
		if apiErr.Code == hcloudgo.ErrorCodeInvalidInput {
			return strings.Contains(strings.ToLower(apiErr.Message), "not assigned")
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "not assigned")
}

// IsPrimaryIPNotAssigned reports whether unassign was called when the primary IP has no assignee.
func IsPrimaryIPNotAssigned(err error) bool {
	if apiErr, ok := AsAPIError(err); ok {
		if apiErr.Code == hcloudgo.ErrorCodeInvalidInput {
			return strings.Contains(strings.ToLower(apiErr.Message), "not assigned")
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "not assigned")
}
