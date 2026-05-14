package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func runAPITokensHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	leader := h.Leader
	email := "apimat-ha-token-user@example.com"

	if err := leader.CreateUser(ctx, &client.CreateUserOptions{
		Email:    email,
		RoleID:   client.RoleReadOnly,
		Password: "ApiMatrixPassw0rd!",
	}); err != nil {
		t.Fatalf("create dep user %q: %v", email, err)
	}

	t.Cleanup(func() {
		if err := leader.DeleteUser(ctx, &client.DeleteUserOptions{Email: email}); err != nil {
			t.Logf("cleanup: delete dep user %q: %v", email, err)
		}
	})

	awaitConvergence(ctx, t, h)

	listAllOn := func(c *client.Client) *client.ListAPITokensResponse {
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

	expiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second).Format(time.RFC3339)

	cases := []struct {
		name         string
		tokenName    string
		createWriter int
		deleteWriter int
		opts         *client.CreateAPITokenOptions
		wantExpiry   string
	}{
		{
			name:         "no_expiry",
			tokenName:    "apimat-ha-token-noexp",
			createWriter: 0,
			deleteWriter: 1,
			opts:         &client.CreateAPITokenOptions{Name: "apimat-ha-token-noexp"},
			wantExpiry:   "",
		},
		{
			name:         "with_expiry",
			tokenName:    "apimat-ha-token-exp",
			createWriter: 2,
			deleteWriter: 0,
			opts:         &client.CreateAPITokenOptions{Name: "apimat-ha-token-exp", ExpiresAt: expiry},
			wantExpiry:   expiry,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			baseline := listAllOn(leader).TotalCount

			if _, err := nodes[tc.createWriter].CreateUserAPIToken(ctx, email, tc.opts); err != nil {
				t.Fatalf("create api token on node %d: %v", tc.createWriter+1, err)
			}

			awaitConvergence(ctx, t, h)

			var createdID string

			for i, c := range nodes {
				list := listAllOn(c)
				if list.TotalCount != baseline+1 {
					t.Fatalf("node %d count after create: got %d, want %d", i+1, list.TotalCount, baseline+1)
				}

				found := findByName(list.Items, tc.tokenName)
				if found == nil {
					t.Fatalf("node %d list after create missing token name %q", i+1, tc.tokenName)
				}

				if i == 0 {
					createdID = found.ID
				}

				if found.ID != createdID {
					t.Fatalf("node %d returned ID %q, want %q (replicated ID mismatch)", i+1, found.ID, createdID)
				}

				if tc.wantExpiry == "" {
					if found.ExpiresAt != "" {
						t.Fatalf("node %d ExpiresAt: got %q, want empty", i+1, found.ExpiresAt)
					}
				} else {
					gotExp, err := time.Parse(time.RFC3339, found.ExpiresAt)
					if err != nil {
						t.Fatalf("node %d ExpiresAt: not RFC 3339: %q (%v)", i+1, found.ExpiresAt, err)
					}

					wantExp, _ := time.Parse(time.RFC3339, tc.wantExpiry)
					if !gotExp.Equal(wantExp) {
						t.Fatalf("node %d ExpiresAt: got %q, want %q", i+1, found.ExpiresAt, tc.wantExpiry)
					}
				}
			}

			deleted := false

			t.Cleanup(func() {
				if deleted {
					return
				}

				if err := leader.DeleteUserAPIToken(ctx, email, createdID); err != nil {
					t.Logf("cleanup: delete api token %q: %v", createdID, err)
				}
			})

			if err := nodes[tc.deleteWriter].DeleteUserAPIToken(ctx, email, createdID); err != nil {
				t.Fatalf("delete api token on node %d: %v", tc.deleteWriter+1, err)
			}

			deleted = true

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				list := listAllOn(c)
				if list.TotalCount != baseline {
					t.Fatalf("node %d count after delete: got %d, want %d", i+1, list.TotalCount, baseline)
				}

				if findByName(list.Items, tc.tokenName) != nil {
					t.Fatalf("node %d list after delete still contains token name %q", i+1, tc.tokenName)
				}
			}
		})
	}
}
