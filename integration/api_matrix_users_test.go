// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runUsersMatrix uses a scoped email to keep the bootstrap admin user
// (in use by the test client itself) untouched. Role and password each have
// their own endpoint, so each is exercised on its own.
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

	Assert(t, contains(afterCreate.Items, email), fmt.Sprintf("list after create missing %q", email))

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

	t.Run("update_password", func(t *testing.T) {
		const newPassword = "ApiMatrixNewPassw0rd!"

		if err := c.UpdateUserPassword(ctx, email, &client.UpdateUserPasswordOptions{Password: newPassword}); err != nil {
			t.Fatalf("update user password: %v", err)
		}

		// Verify with a separate client so the shared session is untouched.
		// The create password was never newPassword, so a successful login
		// with it proves the change took effect.
		verifier, err := client.New(&client.Config{BaseURL: APIAddress()})
		if err != nil {
			t.Fatalf("new verifier client: %v", err)
		}

		if err := verifier.Login(ctx, &client.LoginOptions{Email: email, Password: newPassword}); err != nil {
			t.Fatalf("login with new password: %v", err)
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
