package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runAPITokensMatrix exercises Create/List/Delete for user API tokens.
// API tokens have no Get or Update verb (server.go:93-95), so this is a
// 3-step matrix:
//
//	List → Create → List(contains) → Delete → List(absent)
//
// The bootstrap creates an admin API token used by the test client
// itself (tester_env_test.go:131); we must not delete it. To isolate
// completely we create a side user and operate on tokens belonging to
// that user via the admin-scoped /users/{email}/api-tokens endpoints.
func runAPITokensMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	email := "apimat-token-user@example.com"
	tokenName := "apimat-token"

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

	baseline := listAll()

	if _, err := c.CreateUserAPIToken(ctx, email, &client.CreateAPITokenOptions{
		Name: tokenName,
	}); err != nil {
		t.Fatalf("create api token: %v", err)
	}

	afterCreate := listAll()

	created := findByName(afterCreate.Items, tokenName)
	if created == nil {
		t.Fatalf("list after create missing token name %q", tokenName)
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

	if err := c.DeleteUserAPIToken(ctx, email, created.ID); err != nil {
		t.Fatalf("delete api token %q: %v", created.ID, err)
	}

	deleted = true

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if findByName(afterDelete.Items, tokenName) != nil {
		t.Fatalf("list after delete still contains token name %q", tokenName)
	}
}
