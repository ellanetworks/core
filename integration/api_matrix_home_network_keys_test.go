package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runHomeNetworkKeysMatrix exercises CRD for home network keys (no
// Update verb in the API — server.go:137-138). The key has no list
// endpoint of its own; keys are exposed as Operator.HomeNetworkKeys.
//
// Shape:
//
//	GetOperator → snapshot HomeNetworkKeys
//	Create
//	GetOperator → find created key, capture UUID
//	GetHomeNetworkKeyPrivateKey(uuid) → round-trip private key
//	Delete(uuid)
//	GetOperator → assert key gone
//
// The bootstrap does not provision a home network key, so any existing
// keys belong to a leaked prior run; strict-create surfaces collisions
// on KeyIdentifier+Scheme.
func runHomeNetworkKeysMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const (
		keyIdentifier = 42
		scheme        = "A"
		// X25519 private key (scheme A). Borrowed from
		// internal/tester/scenarios/ue/registration_success_profile_a.go:41.
		privateKey = "c53c22208b61860b06c62e5406a7b330c2b577aab3cd7cd2d3fa33ef6b3df3f6"
	)

	findKey := func(items []client.HomeNetworkKeyResponse, keyID int, sch string) *client.HomeNetworkKeyResponse {
		for i := range items {
			if items[i].KeyIdentifier == keyID && items[i].Scheme == sch {
				return &items[i]
			}
		}

		return nil
	}

	op, err := c.GetOperator(ctx)
	if err != nil {
		t.Fatalf("get operator (baseline): %v", err)
	}

	baselineCount := len(op.HomeNetworkKeys)

	if err := c.CreateHomeNetworkKey(ctx, &client.CreateHomeNetworkKeyOptions{
		KeyIdentifier: keyIdentifier,
		Scheme:        scheme,
		PrivateKey:    privateKey,
	}); err != nil {
		t.Fatalf("create home network key: %v", err)
	}

	op, err = c.GetOperator(ctx)
	if err != nil {
		t.Fatalf("get operator after create: %v", err)
	}

	created := findKey(op.HomeNetworkKeys, keyIdentifier, scheme)
	if created == nil {
		t.Fatalf("home network keys after create missing keyIdentifier=%d scheme=%s", keyIdentifier, scheme)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteHomeNetworkKey(ctx, created.ID); err != nil {
			t.Logf("cleanup: delete home network key %q: %v", created.ID, err)
		}
	})

	if len(op.HomeNetworkKeys) != baselineCount+1 {
		t.Fatalf("home network keys count after create: got %d, want %d", len(op.HomeNetworkKeys), baselineCount+1)
	}

	if created.ID == "" {
		t.Fatalf("home network key returned without an ID")
	}

	pk, err := c.GetHomeNetworkKeyPrivateKey(ctx, created.ID)
	if err != nil {
		t.Fatalf("get home network key private key: %v", err)
	}

	if pk.PrivateKey != privateKey {
		t.Fatalf("private key round-trip mismatch: got %q, want %q", pk.PrivateKey, privateKey)
	}

	if err := c.DeleteHomeNetworkKey(ctx, created.ID); err != nil {
		t.Fatalf("delete home network key %q: %v", created.ID, err)
	}

	deleted = true

	op, err = c.GetOperator(ctx)
	if err != nil {
		t.Fatalf("get operator after delete: %v", err)
	}

	if len(op.HomeNetworkKeys) != baselineCount {
		t.Fatalf("home network keys count after delete: got %d, want %d", len(op.HomeNetworkKeys), baselineCount)
	}

	if findKey(op.HomeNetworkKeys, keyIdentifier, scheme) != nil {
		t.Fatalf("home network keys after delete still contains keyIdentifier=%d scheme=%s", keyIdentifier, scheme)
	}

	_, err = c.GetHomeNetworkKeyPrivateKey(ctx, created.ID)
	assertNotFound(t, err, "home network key after delete")
}
