package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runUsersMatrix exercises full CRUD for users. Only the role_id is
// settable via Update (internal/api/server/api_users.go:26-28); password
// changes go through a dedicated endpoint and are out of scope here.
//
// The bootstrap admin (admin@ellanetworks.com from
// tester_env_test.go:125) must not be touched, so we use a distinct
// scoped email.
func runUsersMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	email := "apimat-user@example.com"

	listAll := func() *client.ListUsersResponse {
		resp, err := c.ListUsers(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list users: %v", err)
		}

		return resp
	}

	contains := func(items []client.User, email string) bool {
		for _, u := range items {
			if u.Email == email {
				return true
			}
		}

		return false
	}

	baseline := listAll()

	createOpts := &client.CreateUserOptions{
		Email:    email,
		RoleID:   client.RoleReadOnly,
		Password: "ApiMatrixPassw0rd!",
	}

	if err := c.CreateUser(ctx, createOpts); err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteUser(ctx, &client.DeleteUserOptions{Email: email}); err != nil {
			t.Logf("cleanup: delete user %q: %v", email, err)
		}
	})

	got, err := c.GetUser(ctx, &client.GetUserOptions{Email: email})
	if err != nil {
		t.Fatalf("get user %q after create: %v", email, err)
	}

	if got.Email != email || got.RoleID != createOpts.RoleID {
		t.Fatalf("post-create round-trip mismatch: got %+v, want email=%s role=%d", got, email, createOpts.RoleID)
	}

	afterCreate := listAll()
	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if !contains(afterCreate.Items, email) {
		t.Fatalf("list after create missing %q", email)
	}

	t.Run("update_RoleID", func(t *testing.T) {
		if err := c.UpdateUser(ctx, email, &client.UpdateUserOptions{RoleID: client.RoleNetworkManager}); err != nil {
			t.Fatalf("update user: %v", err)
		}

		updated, err := c.GetUser(ctx, &client.GetUserOptions{Email: email})
		if err != nil {
			t.Fatalf("get user after update: %v", err)
		}

		if updated.RoleID != client.RoleNetworkManager {
			t.Fatalf("RoleID: got %d, want %d", updated.RoleID, client.RoleNetworkManager)
		}
	})

	if err := c.DeleteUser(ctx, &client.DeleteUserOptions{Email: email}); err != nil {
		t.Fatalf("delete user %q: %v", email, err)
	}

	deleted = true

	_, err = c.GetUser(ctx, &client.GetUserOptions{Email: email})
	assertNotFound(t, err, "user after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if contains(afterDelete.Items, email) {
		t.Fatalf("list after delete still contains %q", email)
	}
}
