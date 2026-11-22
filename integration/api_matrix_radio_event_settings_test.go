// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runRadioEventSettingsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	orig, err := c.GetRadioEventSettings(ctx)
	if err != nil {
		t.Fatalf("get radio event settings (baseline): %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateRadioEventSettings(ctx, &client.UpdateRadioEventSettingsOptions{Enabled: orig.Enabled}); err != nil {
			t.Logf("cleanup: restore radio event settings: %v", err)
		}
	})

	target := !orig.Enabled

	if err := c.UpdateRadioEventSettings(ctx, &client.UpdateRadioEventSettingsOptions{Enabled: target}); err != nil {
		t.Fatalf("update radio event settings: %v", err)
	}

	got, err := c.GetRadioEventSettings(ctx)
	if err != nil {
		t.Fatalf("get radio event settings after update: %v", err)
	}

	if got.Enabled != target {
		t.Fatalf("Enabled: got %t, want %t", got.Enabled, target)
	}
}

func runRadioEventSettingsHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const writerIdx = 1

	orig, err := h.Leader.GetRadioEventSettings(ctx)
	if err != nil {
		t.Fatalf("get radio event settings (baseline): %v", err)
	}

	t.Cleanup(func() {
		if err := h.Leader.UpdateRadioEventSettings(ctx, &client.UpdateRadioEventSettingsOptions{Enabled: orig.Enabled}); err != nil {
			t.Logf("cleanup: restore radio event settings: %v", err)
		}
	})

	target := !orig.Enabled

	if err := h.Clients[writerIdx].UpdateRadioEventSettings(ctx, &client.UpdateRadioEventSettingsOptions{Enabled: target}); err != nil {
		t.Fatalf("update radio event settings on node %d: %v", writerIdx+1, err)
	}

	awaitConvergence(ctx, t, h)

	for i, c := range h.Clients {
		got, err := c.GetRadioEventSettings(ctx)
		if err != nil {
			t.Fatalf("node %d get radio event settings after update: %v", i+1, err)
		}

		if got.Enabled != target {
			t.Fatalf("node %d radio event settings Enabled: got %t, want %t", i+1, got.Enabled, target)
		}
	}
}
