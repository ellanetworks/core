package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runN3InterfaceMatrix round-trips the only mutable N3 field,
// external_address.
func runN3InterfaceMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	orig, err := c.ListNetworkInterfaces(ctx)
	if err != nil {
		t.Fatalf("list interfaces (baseline): %v", err)
	}

	origExternal := orig.N3.ExternalAddress

	t.Cleanup(func() {
		if err := c.UpdateN3Interface(ctx, &client.UpdateN3InterfaceOptions{ExternalAddress: origExternal}); err != nil {
			t.Logf("cleanup: restore n3 external_address: %v", err)
		}
	})

	target := "10.99.99.99"
	if origExternal == target {
		target = "10.99.99.100"
	}

	if err := c.UpdateN3Interface(ctx, &client.UpdateN3InterfaceOptions{ExternalAddress: target}); err != nil {
		t.Fatalf("update n3 external_address: %v", err)
	}

	got, err := c.ListNetworkInterfaces(ctx)
	if err != nil {
		t.Fatalf("list interfaces after update: %v", err)
	}

	if got.N3.ExternalAddress != target {
		t.Fatalf("N3.ExternalAddress: got %q, want %q", got.N3.ExternalAddress, target)
	}
}
