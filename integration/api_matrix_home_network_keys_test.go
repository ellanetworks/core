package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runHomeNetworkKeysMatrix covers both supported SUCI schemes: A
// (X25519) and B (P-256). Keys are listed via Operator.HomeNetworkKeys
// rather than a dedicated endpoint, and have no Update verb.
func runHomeNetworkKeysMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	cases := []struct {
		name          string
		keyIdentifier int // distinct per case to avoid strict-create collisions
		scheme        string
		privateKey    string // 32 bytes hex
	}{
		{
			name:          "scheme_A_x25519",
			keyIdentifier: 42,
			scheme:        "A",
			privateKey:    "c53c22208b61860b06c62e5406a7b330c2b577aab3cd7cd2d3fa33ef6b3df3f6",
		},
		{
			name:          "scheme_B_p256",
			keyIdentifier: 43,
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
			op, err := c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator (baseline): %v", err)
			}

			baselineCount := len(op.HomeNetworkKeys)

			if err := c.CreateHomeNetworkKey(ctx, &client.CreateHomeNetworkKeyOptions{
				KeyIdentifier: tc.keyIdentifier,
				Scheme:        tc.scheme,
				PrivateKey:    tc.privateKey,
			}); err != nil {
				t.Fatalf("create home network key: %v", err)
			}

			op, err = c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator after create: %v", err)
			}

			created := findKey(op.HomeNetworkKeys, tc.keyIdentifier, tc.scheme)
			if created == nil {
				t.Fatalf("home network keys after create missing keyIdentifier=%d scheme=%s", tc.keyIdentifier, tc.scheme)
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

			// PublicKey is derived from PrivateKey by the server.
			if created.PublicKey == "" {
				t.Fatalf("PublicKey: got empty, want non-empty")
			}

			if err := assertPublicKeyForScheme(tc.scheme, created.PublicKey); err != nil {
				t.Fatalf("PublicKey: %v", err)
			}

			pk, err := c.GetHomeNetworkKeyPrivateKey(ctx, created.ID)
			if err != nil {
				t.Fatalf("get home network key private key: %v", err)
			}

			if pk.PrivateKey != tc.privateKey {
				t.Fatalf("private key round-trip mismatch: got %q, want %q", pk.PrivateKey, tc.privateKey)
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

			if findKey(op.HomeNetworkKeys, tc.keyIdentifier, tc.scheme) != nil {
				t.Fatalf("home network keys after delete still contains keyIdentifier=%d scheme=%s", tc.keyIdentifier, tc.scheme)
			}

			_, err = c.GetHomeNetworkKeyPrivateKey(ctx, created.ID)
			assertNotFound(t, err, "home network key after delete")
		})
	}
}

// assertPublicKeyForScheme checks the hex length of the server-returned
// public key. X25519 is 32 bytes (64 hex chars); P-256 is 33 bytes
// compressed (66 hex) or 65 bytes uncompressed (130 hex).
func assertPublicKeyForScheme(scheme, pub string) error {
	switch scheme {
	case "A":
		if len(pub) != 64 {
			return errInvalidPublicKeyLen{scheme: scheme, want: 64, got: len(pub)}
		}
	case "B":
		if len(pub) != 130 && len(pub) != 66 {
			return errInvalidPublicKeyLen{scheme: scheme, want: 130, got: len(pub)}
		}
	}

	return nil
}

type errInvalidPublicKeyLen struct {
	scheme    string
	want, got int
}

func (e errInvalidPublicKeyLen) Error() string {
	return "scheme " + e.scheme + ": unexpected public key hex length"
}
