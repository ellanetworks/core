package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runHomeNetworkKeysMatrix exercises CRD for home network keys. Both
// supported schemes are covered:
//
//   - "A" (X25519, 32-byte private key)
//   - "B" (P-256, 32-byte private key)
//
// For each scheme: create → re-read operator → assert key visible and
// the server-derived PublicKey is populated → fetch the private key by
// UUID → delete → assert gone.
//
// Home network keys have no own list endpoint (exposed via
// Operator.HomeNetworkKeys) and no Update verb (server.go:137-138).
func runHomeNetworkKeysMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	// Cases use distinct KeyIdentifier values so they don't collide with
	// each other or with leaked prior-run state (strict-create surfaces
	// collisions per api_home_network_keys.go:155-159).
	cases := []struct {
		name          string
		keyIdentifier int
		scheme        string
		privateKey    string // 32 bytes hex
	}{
		{
			name:          "scheme_A_x25519",
			keyIdentifier: 42,
			scheme:        "A",
			// From internal/tester/scenarios/ue/registration_success_profile_a.go:41.
			privateKey: "c53c22208b61860b06c62e5406a7b330c2b577aab3cd7cd2d3fa33ef6b3df3f6",
		},
		{
			name:          "scheme_B_p256",
			keyIdentifier: 43,
			scheme:        "B",
			// From internal/api/server/api_home_network_keys_test.go:161.
			privateKey: "f1ab1074477ebcce59b97460c83b4071db578ffab54ee4fbc76aeca38e4b7b01",
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

			// The server derives PublicKey from PrivateKey. Both schemes
			// produce non-empty hex-encoded public keys; locking in
			// "non-empty + correct decoded length" guards against a
			// derivation regression without coupling the test to the
			// crypto implementation.
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

// assertPublicKeyForScheme verifies the server-returned hex-encoded
// public key has the expected length for the scheme. X25519 public keys
// are 32 bytes (64 hex chars); compressed P-256 public keys are 33 bytes
// (66 hex chars) and uncompressed are 65 bytes (130 hex chars). The
// server uses Go's ecdh package which emits the uncompressed form for
// P-256.
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
