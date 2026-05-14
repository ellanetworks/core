package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runFlowAccountingMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	orig, err := c.GetFlowAccountingInfo(ctx)
	if err != nil {
		t.Fatalf("get flow accounting (baseline): %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateFlowAccountingInfo(ctx, &client.UpdateFlowAccountingInfoOptions{Enabled: orig.Enabled}); err != nil {
			t.Logf("cleanup: restore flow accounting: %v", err)
		}
	})

	target := !orig.Enabled

	if err := c.UpdateFlowAccountingInfo(ctx, &client.UpdateFlowAccountingInfoOptions{Enabled: target}); err != nil {
		t.Fatalf("update flow accounting: %v", err)
	}

	got, err := c.GetFlowAccountingInfo(ctx)
	if err != nil {
		t.Fatalf("get flow accounting after update: %v", err)
	}

	if got.Enabled != target {
		t.Fatalf("Enabled: got %t, want %t", got.Enabled, target)
	}
}
