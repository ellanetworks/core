package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runNATHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const writerIdx = 1

	writer := h.Clients[writerIdx]

	baselines := make(map[int]bool, len(h.Clients))

	for i, c := range h.Clients {
		orig, err := c.GetNATInfo(ctx)
		if err != nil {
			t.Fatalf("node %d get nat baseline: %v", i+1, err)
		}

		baselines[i] = orig.Enabled
	}

	t.Cleanup(func() {
		if err := writer.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: baselines[writerIdx]}); err != nil {
			t.Logf("cleanup: restore nat on writer: %v", err)
		}
	})

	target := !baselines[writerIdx]

	if err := writer.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: target}); err != nil {
		t.Fatalf("update nat on writer: %v", err)
	}

	got, err := writer.GetNATInfo(ctx)
	if err != nil {
		t.Fatalf("get nat on writer after update: %v", err)
	}

	if got.Enabled != target {
		t.Fatalf("writer NAT Enabled: got %t, want %t", got.Enabled, target)
	}

	stabilizeLocal()

	for i, c := range h.Clients {
		if i == writerIdx {
			continue
		}

		other, err := c.GetNATInfo(ctx)
		if err != nil {
			t.Fatalf("node %d get nat after writer update: %v", i+1, err)
		}

		if other.Enabled != baselines[i] {
			t.Fatalf("node %d NAT Enabled drifted: got %t, want %t (was set on node %d only)",
				i+1, other.Enabled, baselines[i], writerIdx+1)
		}
	}
}
