// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSingleSignOnBeforeAndAfterV29(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 28)

	if _, err := d.CreateUser(ctx, &User{Email: "admin@example.com", RoleID: RoleAdmin, HashedPassword: "hash"}); err != nil {
		t.Fatalf("CreateUser at schema 28: %v", err)
	}

	if _, err := d.CreateUser(ctx, &User{Email: "nm@example.com", RoleID: RoleNetworkManager, HashedPassword: "hash"}); err != nil {
		t.Fatalf("CreateUser at schema 28: %v", err)
	}

	admin, err := d.GetUser(ctx, "admin@example.com")
	if err != nil || admin.RolePinned || admin.HasIdentity {
		t.Fatalf("GetUser at schema 28 = %+v, %v", admin, err)
	}

	session := &Session{UserID: admin.ID, TokenHash: []byte("token"), CreatedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	if err := d.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession at schema 28: %v", err)
	}

	if s, err := d.GetSessionByID(ctx, session.ID); err != nil || s.Method != SessionPassword || s.IdentityID != "" {
		t.Fatalf("GetSessionByID at schema 28 = %+v, %v", s, err)
	}

	if o, err := d.GetOIDC(ctx); err != nil || o.Enabled || o.SessionLifetime != DefaultOIDCSessionLifetime {
		t.Fatalf("GetOIDC at schema 28 = %+v, %v; want the defaults", o, err)
	}

	if d.SessionsBound(ctx) {
		t.Fatal("sessions are bound at schema 28")
	}

	if err := d.UpdateUser(ctx, "nm@example.com", RoleReadOnly, false); err != nil {
		t.Fatalf("UpdateUser at schema 28: %v", err)
	}

	if err := d.UpdateUser(ctx, "admin@example.com", RoleReadOnly, false); !errors.Is(err, ErrLastPasswordAdmin) {
		t.Fatalf("demote the last password admin at schema 28: err = %v, want ErrLastPasswordAdmin", err)
	}

	writes := map[string]func() error{
		"UpdateUser pinned": func() error { return d.UpdateUser(ctx, "nm@example.com", RoleAdmin, true) },
		"CreateUser pinned": func() error {
			_, err := d.CreateUser(ctx, &User{Email: "p@example.com", RoleID: RoleAdmin, RolePinned: true})
			return err
		},
		"UpdateOIDC":   func() error { return d.UpdateOIDC(ctx, DefaultOIDC()) },
		"UnlinkUser":   func() error { return d.UnlinkUser(ctx, "nm@example.com") },
		"UseOIDCState": func() error { return d.UseOIDCState(ctx, make([]byte, 32), 1, 0) },
		"LogInWithOIDC": func() error {
			_, err := d.LogInWithOIDC(ctx, &OIDCLogin{})
			return err
		},
	}

	for name, write := range writes {
		if err := write(); !errors.Is(err, ErrMigrationPending) {
			t.Errorf("%s at schema 28: err = %v, want ErrMigrationPending", name, err)
		}
	}

	if err := runMigrations(ctx, d.connection, 29); err != nil {
		t.Fatalf("migrate to 29: %v", err)
	}

	d.appliedSchemaCache.Store(0)

	if !d.SessionsBound(ctx) {
		t.Fatal("sessions are not bound at schema 29")
	}

	if s, err := d.GetSessionByID(ctx, session.ID); err != nil || s.Method != SessionPassword || s.IdentityID != "" {
		t.Fatalf("migrated session = %+v, %v", s, err)
	}

	if u, err := d.GetUser(ctx, "nm@example.com"); err != nil || u.RolePinned || u.HasIdentity || u.RoleID != RoleReadOnly {
		t.Fatalf("migrated user = %+v, %v", u, err)
	}

	if err := d.UpdateUser(ctx, "nm@example.com", RoleAdmin, true); err != nil {
		t.Fatalf("UpdateUser pinned at schema 29: %v", err)
	}
}
