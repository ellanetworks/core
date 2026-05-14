package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runHomeNetworkKeysHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	cases := []struct {
		name          string
		createWriter  int
		deleteWriter  int
		keyIdentifier int
		scheme        string
		privateKey    string
	}{
		{
			name:          "scheme_A_x25519",
			createWriter:  0,
			deleteWriter:  1,
			keyIdentifier: 142,
			scheme:        "A",
			privateKey:    "c53c22208b61860b06c62e5406a7b330c2b577aab3cd7cd2d3fa33ef6b3df3f6",
		},
		{
			name:          "scheme_B_p256",
			createWriter:  2,
			deleteWriter:  0,
			keyIdentifier: 143,
			scheme:        "B",
			privateKey:    "f1ab1074477ebcce59b97460c83b4071db578ffab54ee4fbc76aeca38e4b7b01",
		},
	}

	findKey := func(items []client.HomeNetworkKeyResponse, keyID int, scheme string) *client.HomeNetworkKeyResponse {
		for i := range items {
			if items[i].KeyIdentifier == keyID && items[i].Scheme == scheme {
				return &items[i]
			}
		}

		return nil
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			baselineOp, err := h.Leader.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator (baseline): %v", err)
			}

			baselineCount := len(baselineOp.HomeNetworkKeys)

			if err := nodes[tc.createWriter].CreateHomeNetworkKey(ctx, &client.CreateHomeNetworkKeyOptions{
				KeyIdentifier: tc.keyIdentifier,
				Scheme:        tc.scheme,
				PrivateKey:    tc.privateKey,
			}); err != nil {
				t.Fatalf("create home network key on node %d: %v", tc.createWriter+1, err)
			}

			awaitConvergence(ctx, t, h)

			var createdID string

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after create: %v", i+1, err)
				}

				if len(op.HomeNetworkKeys) != baselineCount+1 {
					t.Fatalf("node %d count after create: got %d, want %d",
						i+1, len(op.HomeNetworkKeys), baselineCount+1)
				}

				created := findKey(op.HomeNetworkKeys, tc.keyIdentifier, tc.scheme)
				if created == nil {
					t.Fatalf("node %d operator missing keyIdentifier=%d scheme=%s", i+1, tc.keyIdentifier, tc.scheme)
				}

				if created.ID == "" {
					t.Fatalf("node %d returned home network key without an ID", i+1)
				}

				if i == 0 {
					createdID = created.ID
				}

				if created.ID != createdID {
					t.Fatalf("node %d returned ID %q, want %q (replicated UUID mismatch)", i+1, created.ID, createdID)
				}

				if created.PublicKey == "" {
					t.Fatalf("node %d PublicKey: got empty, want non-empty", i+1)
				}

				if err := assertPublicKeyForScheme(tc.scheme, created.PublicKey); err != nil {
					t.Fatalf("node %d PublicKey: %v", i+1, err)
				}
			}

			deleted := false

			t.Cleanup(func() {
				if deleted {
					return
				}

				if err := h.Leader.DeleteHomeNetworkKey(ctx, createdID); err != nil {
					t.Logf("cleanup: delete home network key %q: %v", createdID, err)
				}
			})

			for i, c := range nodes {
				pk, err := c.GetHomeNetworkKeyPrivateKey(ctx, createdID)
				if err != nil {
					t.Fatalf("node %d get home network key private key: %v", i+1, err)
				}

				if pk.PrivateKey != tc.privateKey {
					t.Fatalf("node %d private key round-trip mismatch: got %q, want %q",
						i+1, pk.PrivateKey, tc.privateKey)
				}
			}

			if err := nodes[tc.deleteWriter].DeleteHomeNetworkKey(ctx, createdID); err != nil {
				t.Fatalf("delete home network key on node %d: %v", tc.deleteWriter+1, err)
			}

			deleted = true

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after delete: %v", i+1, err)
				}

				if len(op.HomeNetworkKeys) != baselineCount {
					t.Fatalf("node %d count after delete: got %d, want %d",
						i+1, len(op.HomeNetworkKeys), baselineCount)
				}

				if findKey(op.HomeNetworkKeys, tc.keyIdentifier, tc.scheme) != nil {
					t.Fatalf("node %d operator after delete still contains keyIdentifier=%d scheme=%s",
						i+1, tc.keyIdentifier, tc.scheme)
				}

				_, err = c.GetHomeNetworkKeyPrivateKey(ctx, createdID)
				assertNotFound(t, err, fmt.Sprintf("home network key on node %d after delete", i+1))
			}
		})
	}
}
