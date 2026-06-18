package e2e

import (
	"strings"
	"testing"
)

func TestResourceName(t *testing.T) {
	got := ResourceName("hkic-e2e-42", "server")
	want := "hkic-e2e-42-server"
	if got != want {
		t.Fatalf("ResourceName() = %q, want %q", got, want)
	}
}

func TestNamePrefixUsesGitHubRunID(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "123456789")
	got := NamePrefix()
	if got != "hkic-e2e-123456789" {
		t.Fatalf("NamePrefix() = %q, want hkic-e2e-123456789", got)
	}
}

func TestNamePrefixLocalFallback(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "")
	got := NamePrefix()
	if !strings.HasPrefix(got, "hkic-e2e-") {
		t.Fatalf("NamePrefix() = %q, want prefix hkic-e2e-", got)
	}
}
