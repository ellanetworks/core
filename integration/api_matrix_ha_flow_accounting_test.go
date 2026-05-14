package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runFlowAccountingHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const writerIdx = 2

	writer := h.Clients[writerIdx]

	baselines := make(map[int]bool, len(h.Clients))

	for i, c := range h.Clients {
		orig, err := c.GetFlowAccountingInfo(ctx)
		if err != nil {
			t.Fatalf("node %d get flow accounting baseline: %v", i+1, err)
		}

		baselines[i] = orig.Enabled
	}

	t.Cleanup(func() {
		if err := writer.UpdateFlowAccountingInfo(ctx, &client.UpdateFlowAccountingInfoOptions{Enabled: baselines[writerIdx]}); err != nil {
			t.Logf("cleanup: restore flow accounting on writer: %v", err)
		}
	})

	target := !baselines[writerIdx]

	if err := writer.UpdateFlowAccountingInfo(ctx, &client.UpdateFlowAccountingInfoOptions{Enabled: target}); err != nil {
		t.Fatalf("update flow accounting on writer: %v", err)
	}

	got, err := writer.GetFlowAccountingInfo(ctx)
	if err != nil {
		t.Fatalf("get flow accounting on writer after update: %v", err)
	}

	if got.Enabled != target {
		t.Fatalf("writer flow accounting Enabled: got %t, want %t", got.Enabled, target)
	}

	stabilizeLocal()

	for i, c := range h.Clients {
		if i == writerIdx {
			continue
		}

		other, err := c.GetFlowAccountingInfo(ctx)
		if err != nil {
			t.Fatalf("node %d get flow accounting after writer update: %v", i+1, err)
		}

		if other.Enabled != baselines[i] {
			t.Fatalf("node %d flow accounting Enabled drifted: got %t, want %t (was set on node %d only)",
				i+1, other.Enabled, baselines[i], writerIdx+1)
		}
	}
}
