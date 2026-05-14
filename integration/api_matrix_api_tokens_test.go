package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// runAPITokensMatrix exercises Create/List/Delete for user API tokens.
// API tokens have no Get or Update verb (server.go:93-95), so this is a
// 3-step matrix:
//
//	List → Create → List(contains) → Delete → List(absent)
//
// Two sub-cases are run on the same backing user — one with no expiry,
// one with an explicit RFC 3339 ExpiresAt — to round-trip the optional
// field. The bootstrap creates an admin API token used by the test
// client itself (tester_env_test.go:131); we must not delete it. To
// isolate completely we create a side user and operate on tokens
// belonging to that user via the admin-scoped /users/{email}/api-tokens
// endpoints.
func runAPITokensMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	email := "apimat-token-user@example.com"

	if err := c.CreateUser(ctx, &client.CreateUserOptions{
		Email:    email,
		RoleID:   client.RoleReadOnly,
		Password: "ApiMatrixPassw0rd!",
	}); err != nil {
		t.Fatalf("create dep user %q: %v", email, err)
	}

	t.Cleanup(func() {
		if err := c.DeleteUser(ctx, &client.DeleteUserOptions{Email: email}); err != nil {
			t.Logf("cleanup: delete dep user %q: %v", email, err)
		}
	})

	listAll := func() *client.ListAPITokensResponse {
		resp, err := c.ListUserAPITokens(ctx, email, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list api tokens for %q: %v", email, err)
		}

		return resp
	}

	findByName := func(items []client.APIToken, name string) *client.APIToken {
		for i := range items {
			if items[i].Name == name {
				return &items[i]
			}
		}

		return nil
	}

	// Server expects RFC 3339 (api_users.go:663 calls time.Parse(time.RFC3339, ...)).
	expiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second).Format(time.RFC3339)

	cases := []struct {
		name       string
		tokenName  string
		opts       *client.CreateAPITokenOptions
		wantExpiry string // empty means "must be empty on the list response"
	}{
		{
			name:       "no_expiry",
			tokenName:  "apimat-token-noexp",
			opts:       &client.CreateAPITokenOptions{Name: "apimat-token-noexp"},
			wantExpiry: "",
		},
		{
			name:       "with_expiry",
			tokenName:  "apimat-token-exp",
			opts:       &client.CreateAPITokenOptions{Name: "apimat-token-exp", ExpiresAt: expiry},
			wantExpiry: expiry,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			baseline := listAll()

			if _, err := c.CreateUserAPIToken(ctx, email, tc.opts); err != nil {
				t.Fatalf("create api token: %v", err)
			}

			afterCreate := listAll()

			created := findByName(afterCreate.Items, tc.tokenName)
			if created == nil {
				t.Fatalf("list after create missing token name %q", tc.tokenName)
			}

			deleted := false

			t.Cleanup(func() {
				if deleted {
					return
				}

				if err := c.DeleteUserAPIToken(ctx, email, created.ID); err != nil {
					t.Logf("cleanup: delete api token %q: %v", created.ID, err)
				}
			})

			if afterCreate.TotalCount != baseline.TotalCount+1 {
				t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
			}

			// Round-trip the ExpiresAt field. Compare as time.Time to be
			// tolerant of timezone formatting normalization on the server
			// side (e.g. "Z" vs "+00:00") while still pinning the moment.
			if tc.wantExpiry == "" {
				if created.ExpiresAt != "" {
					t.Fatalf("ExpiresAt: got %q, want empty", created.ExpiresAt)
				}
			} else {
				got, err := time.Parse(time.RFC3339, created.ExpiresAt)
				if err != nil {
					t.Fatalf("ExpiresAt: not RFC 3339: %q (%v)", created.ExpiresAt, err)
				}

				want, _ := time.Parse(time.RFC3339, tc.wantExpiry)
				if !got.Equal(want) {
					t.Fatalf("ExpiresAt: got %q, want %q", created.ExpiresAt, tc.wantExpiry)
				}
			}

			if err := c.DeleteUserAPIToken(ctx, email, created.ID); err != nil {
				t.Fatalf("delete api token %q: %v", created.ID, err)
			}

			deleted = true

			afterDelete := listAll()
			if afterDelete.TotalCount != baseline.TotalCount {
				t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
			}

			if findByName(afterDelete.Items, tc.tokenName) != nil {
				t.Fatalf("list after delete still contains token name %q", tc.tokenName)
			}
		})
	}
}
