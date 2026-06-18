package e2e

import (
	"fmt"
	"os"
	"time"
)

const namePrefix = "hkic-e2e"

// NamePrefix returns a unique prefix for real-Hetzner E2E resources.
// Uses GITHUB_RUN_ID in CI; falls back to a unix timestamp locally.
func NamePrefix() string {
	if id := os.Getenv("GITHUB_RUN_ID"); id != "" {
		return fmt.Sprintf("%s-%s", namePrefix, id)
	}
	return fmt.Sprintf("%s-%d", namePrefix, time.Now().Unix())
}

// ResourceName joins a prefix and resource suffix with a single hyphen.
func ResourceName(prefix, resource string) string {
	return prefix + "-" + resource
}
