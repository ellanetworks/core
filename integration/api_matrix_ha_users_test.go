package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runUsersHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	email := "apimat-ha-user@example.com"

	listAllOn := func(c *client.Client) *client.ListUsersResponse {
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

	baseline := listAllOn(h.Leader).TotalCount

	createOpts := &client.CreateUserOptions{
		Email:    email,
		RoleID:   client.RoleReadOnly,
		Password: "ApiMatrixPassw0rd!",
	}

	if err := nodes[0].CreateUser(ctx, createOpts); err != nil {
		t.Fatalf("create user on node 1: %v", err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := h.Leader.DeleteUser(ctx, &client.DeleteUserOptions{Email: email}); err != nil {
			t.Logf("cleanup: delete user: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetUser(ctx, &client.GetUserOptions{Email: email})
		if err != nil {
			t.Fatalf("node %d get after create: %v", i+1, err)
		}

		if got.Email != email || got.RoleID != createOpts.RoleID {
			t.Fatalf("node %d post-create mismatch: got %+v, want email=%s role=%d",
				i+1, got, email, createOpts.RoleID)
		}

		list := listAllOn(c)
		if list.TotalCount != baseline+1 {
			t.Fatalf("node %d count after create: got %d, want %d", i+1, list.TotalCount, baseline+1)
		}

		if !contains(list.Items, email) {
			t.Fatalf("node %d list after create missing %q", i+1, email)
		}
	}

	t.Run("update_RoleID", func(t *testing.T) {
		if err := nodes[1].UpdateUser(ctx, email, &client.UpdateUserOptions{RoleID: client.RoleNetworkManager}); err != nil {
			t.Fatalf("update user on node 2: %v", err)
		}

		awaitConvergence(ctx, t, h)

		for i, c := range nodes {
			got, err := c.GetUser(ctx, &client.GetUserOptions{Email: email})
			if err != nil {
				t.Fatalf("node %d get after update: %v", i+1, err)
			}

			if got.RoleID != client.RoleNetworkManager {
				t.Fatalf("node %d RoleID: got %d, want %d", i+1, got.RoleID, client.RoleNetworkManager)
			}
		}
	})

	if err := nodes[2].DeleteUser(ctx, &client.DeleteUserOptions{Email: email}); err != nil {
		t.Fatalf("delete user on node 3: %v", err)
	}

	deleted = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		_, err := c.GetUser(ctx, &client.GetUserOptions{Email: email})
		assertNotFound(t, err, fmt.Sprintf("user on node %d after delete", i+1))

		list := listAllOn(c)
		if list.TotalCount != baseline {
			t.Fatalf("node %d count after delete: got %d, want %d", i+1, list.TotalCount, baseline)
		}

		if contains(list.Items, email) {
			t.Fatalf("node %d list after delete still contains %q", i+1, email)
		}
	}
}
