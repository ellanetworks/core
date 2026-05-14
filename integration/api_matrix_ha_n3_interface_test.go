package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runN3InterfaceHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const writerIdx = 0

	writer := h.Clients[writerIdx]

	baselines := make(map[int]string, len(h.Clients))

	for i, c := range h.Clients {
		orig, err := c.ListNetworkInterfaces(ctx)
		if err != nil {
			t.Fatalf("node %d list interfaces baseline: %v", i+1, err)
		}

		baselines[i] = orig.N3.ExternalAddress
	}

	t.Cleanup(func() {
		if err := writer.UpdateN3Interface(ctx, &client.UpdateN3InterfaceOptions{ExternalAddress: baselines[writerIdx]}); err != nil {
			t.Logf("cleanup: restore n3 external_address on writer: %v", err)
		}
	})

	target := "10.99.99.99"
	if baselines[writerIdx] == target {
		target = "10.99.99.100"
	}

	if err := writer.UpdateN3Interface(ctx, &client.UpdateN3InterfaceOptions{ExternalAddress: target}); err != nil {
		t.Fatalf("update n3 external_address on writer: %v", err)
	}

	got, err := writer.ListNetworkInterfaces(ctx)
	if err != nil {
		t.Fatalf("get interfaces on writer after update: %v", err)
	}

	if got.N3.ExternalAddress != target {
		t.Fatalf("writer N3.ExternalAddress: got %q, want %q", got.N3.ExternalAddress, target)
	}

	stabilizeLocal()

	for i, c := range h.Clients {
		if i == writerIdx {
			continue
		}

		other, err := c.ListNetworkInterfaces(ctx)
		if err != nil {
			t.Fatalf("node %d list interfaces after writer update: %v", i+1, err)
		}

		if other.N3.ExternalAddress != baselines[i] {
			t.Fatalf("node %d N3.ExternalAddress drifted: got %q, want %q (was set on node %d only)",
				i+1, other.N3.ExternalAddress, baselines[i], writerIdx+1)
		}
	}
}
