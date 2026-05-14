package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runNATMatrix exercises Get + Update for the NAT singleton. The
// singleton shape captures the original value, mutates, asserts the
// round-trip, and restores via t.Cleanup so subsequent tests see the
// system in its original state. The bootstrap sets Enabled=true
// (tester_env_test.go:140), so the original is typically true.
func runNATMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	orig, err := c.GetNATInfo(ctx)
	if err != nil {
		t.Fatalf("get nat (baseline): %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: orig.Enabled}); err != nil {
			t.Logf("cleanup: restore nat: %v", err)
		}
	})

	target := !orig.Enabled

	if err := c.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: target}); err != nil {
		t.Fatalf("update nat: %v", err)
	}

	got, err := c.GetNATInfo(ctx)
	if err != nil {
		t.Fatalf("get nat after update: %v", err)
	}

	if got.Enabled != target {
		t.Fatalf("Enabled: got %t, want %t", got.Enabled, target)
	}
}
