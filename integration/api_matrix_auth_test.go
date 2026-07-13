// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runAuthMatrix exercises the auth surface end-to-end. The session lifecycle
// (login, refresh, self password change, logout) runs on throwaway clients; the
// shared bootstrap client is used only for the admin create/delete and for the
// token-introspection and rotate-secret steps. The test client authenticates
// with an API token, which is validated independently of the JWT secret, so it
// survives rotate-secret; that is asserted last.
func runAuthMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const (
		email = "apimat-auth-lifecycle@example.com"
		pw1   = "ApiMatrixPassw0rd!"
		pw2   = "ApiMatrixNewPassw0rd!"
	)

	newClient := func() *client.Client {
		cl, err := client.New(&client.Config{BaseURL: APIAddress()})
		if err != nil {
			t.Fatalf("new client: %v", err)
		}

		return cl
	}

	// Login before the user exists.
	if err := newClient().Login(ctx, &client.LoginOptions{Email: email, Password: pw1}); err == nil {
		t.Fatalf("login for non-existent user: expected error, got success")
	}

	if err := c.CreateUser(ctx, &client.CreateUserOptions{
		Email:    email,
		RoleID:   client.RoleReadOnly,
		Password: pw1,
	}); err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}

	t.Cleanup(func() {
		if err := c.DeleteUser(ctx, &client.DeleteUserOptions{Email: email}); err != nil {
			t.Logf("cleanup: delete user %q: %v", email, err)
		}
	})

	// Wrong password is rejected.
	if err := newClient().Login(ctx, &client.LoginOptions{Email: email, Password: "wrong-" + pw1}); err == nil {
		t.Fatalf("login with wrong password: expected error, got success")
	}

	// Log in and establish a session.
	sess := newClient()
	if err := sess.Login(ctx, &client.LoginOptions{Email: email, Password: pw1}); err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := sess.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// A wrong current password is rejected before the real change.
	if err := sess.UpdateMyPassword(ctx, &client.UpdateMyPasswordOptions{CurrentPassword: "wrong", Password: pw2}); err == nil {
		t.Fatalf("update password with wrong current: expected error, got success")
	}

	if err := sess.UpdateMyPassword(ctx, &client.UpdateMyPasswordOptions{CurrentPassword: pw1, Password: pw2}); err != nil {
		t.Fatalf("update my password: %v", err)
	}

	// The old password no longer works; the new one does.
	if err := newClient().Login(ctx, &client.LoginOptions{Email: email, Password: pw1}); err == nil {
		t.Fatalf("login with old password after change: expected error, got success")
	}

	if err := newClient().Login(ctx, &client.LoginOptions{Email: email, Password: pw2}); err != nil {
		t.Fatalf("login with new password: %v", err)
	}

	// Logout ends the session, so a subsequent refresh fails.
	if err := sess.Logout(ctx); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if err := sess.Refresh(ctx); err == nil {
		t.Fatalf("refresh after logout: expected error, got success")
	}

	// The shared client's API token is accepted by token introspection.
	lookup, err := c.LookupToken(ctx)
	if err != nil {
		t.Fatalf("lookup token: %v", err)
	}

	if !lookup.Valid {
		t.Fatalf("lookup token: got invalid, want valid")
	}

	// Rotate the JWT secret last: it invalidates JWT sessions but not API
	// tokens, so the shared client must still be authenticated afterwards.
	if err := c.RotateSecret(ctx); err != nil {
		t.Fatalf("rotate secret: %v", err)
	}

	if _, err := c.GetMyUser(ctx); err != nil {
		t.Fatalf("get my user after rotate-secret (API token invalidated?): %v", err)
	}
}
