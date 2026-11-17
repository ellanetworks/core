// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/google/uuid"
)

const (
	testIssuer   = "https://idp.example.com"
	testClientID = "ella-core"
)

func enableOIDC(t *testing.T, database *db.Database, groupRoles ...db.GroupRole) db.OIDC {
	t.Helper()

	o := db.DefaultOIDC()
	o.Enabled = true
	o.ExternalURL = "https://core.example.com"
	o.Issuer = testIssuer
	o.ClientID = testClientID
	o.ClientSecret = "secret"
	o.DisplayName = "Keycloak"

	if len(groupRoles) > 0 {
		o.GroupsClaim = "groups"
		o.GroupRoles = groupRoles
	}

	if err := database.UpdateOIDC(t.Context(), o); err != nil {
		t.Fatalf("UpdateOIDC: %v", err)
	}

	return o
}

func createUser(t *testing.T, database *db.Database, email string, role db.RoleID, password string) *db.User {
	t.Helper()

	u := &db.User{Email: email, RoleID: role, HashedPassword: password}
	if _, err := database.CreateUser(t.Context(), u); err != nil {
		t.Fatalf("CreateUser %s: %v", email, err)
	}

	return u
}

func newLogin(t *testing.T, subject, email string) *db.OIDCLogin {
	t.Helper()

	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	hash := sha256.Sum256(raw)
	now := time.Now().Unix()

	return &db.OIDCLogin{
		Issuer:             testIssuer,
		ClientID:           testClientID,
		Subject:            subject,
		Email:              email,
		LinkEmail:          email,
		MaxUsers:           50,
		MaxSessionsPerUser: 10,
		NewUserID:          uuid.NewString(),
		NewIdentityID:      uuid.NewString(),
		Session: db.Session{
			ID:        uuid.NewString(),
			TokenHash: hash[:],
			CreatedAt: now,
			ExpiresAt: now + 3600,
		},
	}
}

func TestOIDCSettingsDefaultsAndUpdate(t *testing.T) {
	database := setupTestDB(t)

	o, err := database.GetOIDC(t.Context())
	if err != nil {
		t.Fatalf("GetOIDC: %v", err)
	}

	if o.Enabled || o.SessionLifetime != db.DefaultOIDCSessionLifetime || len(o.Scopes) != 3 || o.GroupRoles == nil {
		t.Fatalf("unexpected defaults: %+v", o)
	}

	want := enableOIDC(t, database, db.GroupRole{Group: "admins", RoleID: db.RoleAdmin}, db.GroupRole{Group: "noc", RoleID: db.RoleReadOnly})

	got, err := database.GetOIDC(t.Context())
	if err != nil {
		t.Fatalf("GetOIDC: %v", err)
	}

	if got.Issuer != want.Issuer || got.ClientSecret != want.ClientSecret || len(got.GroupRoles) != 2 ||
		got.GroupRoles[0].Group != "admins" || got.GroupRoles[1].RoleID != db.RoleReadOnly {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestOIDCLoginLinksExistingUserByEmailInAnyCase(t *testing.T) {
	database := setupTestDB(t)
	enableOIDC(t, database)
	u := createUser(t, database, "Alice@Example.com", db.RoleNetworkManager, "hash")

	res, err := database.LogInWithOIDC(t.Context(), newLogin(t, "sub-alice", "alice@example.com"))
	if err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if !res.Linked || res.Created || res.User.ID != u.ID || res.User.RoleID != db.RoleNetworkManager {
		t.Fatalf("unexpected result: %+v", res)
	}

	got, err := database.GetUser(t.Context(), "Alice@Example.com")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	if !got.HasIdentity {
		t.Fatal("user is not linked")
	}

	s, err := database.GetSessionByID(t.Context(), res.SessionID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}

	if s.Method != db.SessionOIDC || s.IdentityID != res.IdentityID || s.UserID != u.ID {
		t.Fatalf("unexpected session: %+v", s)
	}

	again := newLogin(t, "sub-alice", "renamed@example.com")
	again.LinkEmail = ""

	res, err = database.LogInWithOIDC(t.Context(), again)
	if err != nil {
		t.Fatalf("second LogInWithOIDC: %v", err)
	}

	if res.Linked || res.User.ID != u.ID {
		t.Fatalf("second login did not use the link: %+v", res)
	}
}

func TestOIDCLoginRefusals(t *testing.T) {
	database := setupTestDB(t)
	enableOIDC(t, database)
	createUser(t, database, "bob@example.com", db.RoleReadOnly, "hash")

	unverified := newLogin(t, "sub-x", "bob@example.com")
	unverified.LinkEmail = ""

	if _, err := database.LogInWithOIDC(t.Context(), unverified); !errors.Is(err, db.ErrNotLinked) {
		t.Fatalf("unverified email: got %v, want ErrNotLinked", err)
	}

	if _, err := database.LogInWithOIDC(t.Context(), newLogin(t, "sub-y", "nobody@example.com")); !errors.Is(err, db.ErrNoUser) {
		t.Fatalf("unknown email: got %v, want ErrNoUser", err)
	}

	if _, err := database.LogInWithOIDC(t.Context(), newLogin(t, "sub-bob", "bob@example.com")); err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if _, err := database.LogInWithOIDC(t.Context(), newLogin(t, "sub-other", "bob@example.com")); !errors.Is(err, db.ErrLinkedElsewhere) {
		t.Fatalf("second account: got %v, want ErrLinkedElsewhere", err)
	}

	changed := newLogin(t, "sub-bob", "bob@example.com")
	changed.ClientID = "other"

	if _, err := database.LogInWithOIDC(t.Context(), changed); !errors.Is(err, db.ErrOIDCChanged) {
		t.Fatalf("changed client: got %v, want ErrOIDCChanged", err)
	}

	o, err := database.GetOIDC(t.Context())
	if err != nil {
		t.Fatalf("GetOIDC: %v", err)
	}

	o.Enabled = false
	if err := database.UpdateOIDC(t.Context(), o); err != nil {
		t.Fatalf("UpdateOIDC: %v", err)
	}

	if _, err := database.LogInWithOIDC(t.Context(), newLogin(t, "sub-bob", "bob@example.com")); !errors.Is(err, db.ErrOIDCDisabled) {
		t.Fatalf("disabled: got %v, want ErrOIDCDisabled", err)
	}
}

func TestUserEmailsAreUniqueInAnyCase(t *testing.T) {
	database := setupTestDB(t)
	enableOIDC(t, database)
	createUser(t, database, "carol@example.com", db.RoleReadOnly, "hash")

	if _, err := database.CreateUser(t.Context(), &db.User{Email: "Carol@example.com", RoleID: db.RoleReadOnly, HashedPassword: "hash"}); !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("CreateUser in another case: got %v, want ErrAlreadyExists", err)
	}

	res, err := database.LogInWithOIDC(t.Context(), newLogin(t, "sub-carol", "carol@example.com"))
	if err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if res.User.Email != "carol@example.com" {
		t.Fatalf("linked %s", res.User.Email)
	}
}

func TestOIDCLoginGroupMappings(t *testing.T) {
	database := setupTestDB(t)
	enableOIDC(t, database, db.GroupRole{Group: "noc", RoleID: db.RoleNetworkManager})
	createUser(t, database, "admin@example.com", db.RoleAdmin, "hash")

	l := newLogin(t, "sub-dave", "dave@example.com")
	l.MapsGroups = true

	if _, err := database.LogInWithOIDC(t.Context(), l); !errors.Is(err, db.ErrNoRole) {
		t.Fatalf("no group: got %v, want ErrNoRole", err)
	}

	l = newLogin(t, "sub-dave", "dave@example.com")
	l.MapsGroups, l.RoleID = true, db.RoleNetworkManager

	res, err := database.LogInWithOIDC(t.Context(), l)
	if err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if !res.Created || res.User.RoleID != db.RoleNetworkManager || res.User.Email != "dave@example.com" {
		t.Fatalf("unexpected result: %+v", res)
	}

	u, err := database.GetUser(t.Context(), "dave@example.com")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	if u.HashedPassword != "" || !u.HasIdentity {
		t.Fatalf("unexpected user: %+v", u)
	}

	l = newLogin(t, "sub-dave", "dave@example.com")
	l.MapsGroups, l.RoleID = true, db.RoleReadOnly

	res, err = database.LogInWithOIDC(t.Context(), l)
	if err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if !res.RoleChanged() || res.User.RoleID != db.RoleReadOnly {
		t.Fatalf("role not synced: %+v", res)
	}

	if err := database.UpdateUser(t.Context(), "dave@example.com", db.RoleAdmin, true); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	l = newLogin(t, "sub-dave", "dave@example.com")
	l.MapsGroups, l.RoleID = true, db.RoleReadOnly

	res, err = database.LogInWithOIDC(t.Context(), l)
	if err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if res.User.RoleID != db.RoleAdmin || res.RoleChanged() {
		t.Fatalf("pinned role not kept: %+v", res)
	}

	l = newLogin(t, "sub-erin", "erin@example.com")
	l.MapsGroups, l.RoleID, l.MaxUsers = true, db.RoleReadOnly, 2

	if _, err := database.LogInWithOIDC(t.Context(), l); !errors.Is(err, db.ErrUserLimit) {
		t.Fatalf("user limit: got %v, want ErrUserLimit", err)
	}
}

func TestOIDCLoginKeepsLastPasswordAdmin(t *testing.T) {
	database := setupTestDB(t)
	enableOIDC(t, database, db.GroupRole{Group: "noc", RoleID: db.RoleReadOnly})
	createUser(t, database, "admin@example.com", db.RoleAdmin, "hash")

	l := newLogin(t, "sub-admin", "admin@example.com")
	l.MapsGroups, l.RoleID = true, db.RoleReadOnly

	res, err := database.LogInWithOIDC(t.Context(), l)
	if err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if !res.RoleKept || res.User.RoleID != db.RoleAdmin {
		t.Fatalf("last password admin demoted: %+v", res)
	}
}

func TestLastPasswordAdminGuards(t *testing.T) {
	database := setupTestDB(t)
	createUser(t, database, "admin@example.com", db.RoleAdmin, "hash")
	createUser(t, database, "sso-admin@example.com", db.RoleAdmin, "")

	if err := database.UpdateUser(t.Context(), "admin@example.com", db.RoleReadOnly, false); !errors.Is(err, db.ErrLastPasswordAdmin) {
		t.Fatalf("demote: got %v, want ErrLastPasswordAdmin", err)
	}

	if err := database.UpdateUserPassword(t.Context(), "admin@example.com", ""); !errors.Is(err, db.ErrLastPasswordAdmin) {
		t.Fatalf("remove password: got %v, want ErrLastPasswordAdmin", err)
	}

	if err := database.DeleteUser(t.Context(), "admin@example.com"); !errors.Is(err, db.ErrLastPasswordAdmin) {
		t.Fatalf("delete: got %v, want ErrLastPasswordAdmin", err)
	}

	if err := database.UpdateUserPassword(t.Context(), "sso-admin@example.com", ""); !errors.Is(err, db.ErrNoPassword) {
		t.Fatalf("remove missing password: got %v, want ErrNoPassword", err)
	}

	if err := database.UpdateUser(t.Context(), "sso-admin@example.com", db.RoleReadOnly, false); err != nil {
		t.Fatalf("demote passwordless admin: %v", err)
	}

	createUser(t, database, "second@example.com", db.RoleAdmin, "hash")

	if err := database.DeleteUser(t.Context(), "admin@example.com"); err != nil {
		t.Fatalf("delete with another password admin: %v", err)
	}
}

func TestDisablingOIDCAndUnlinkingEndOIDCSessions(t *testing.T) {
	database := setupTestDB(t)
	o := enableOIDC(t, database)
	createUser(t, database, "frank@example.com", db.RoleReadOnly, "hash")

	res, err := database.LogInWithOIDC(t.Context(), newLogin(t, "sub-frank", "frank@example.com"))
	if err != nil {
		t.Fatalf("LogInWithOIDC: %v", err)
	}

	if err := database.UnlinkUser(t.Context(), "frank@example.com"); err != nil {
		t.Fatalf("UnlinkUser: %v", err)
	}

	if _, err := database.GetSessionByID(t.Context(), res.SessionID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("session after unlink: got %v, want ErrNotFound", err)
	}

	if err := database.UnlinkUser(t.Context(), "frank@example.com"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("second unlink: got %v, want ErrNotFound", err)
	}

	res, err = database.LogInWithOIDC(t.Context(), newLogin(t, "sub-frank-2", "frank@example.com"))
	if err != nil {
		t.Fatalf("LogInWithOIDC after unlink: %v", err)
	}

	if !res.Linked {
		t.Fatal("not linked again")
	}

	o.Enabled = false
	if err := database.UpdateOIDC(t.Context(), o); err != nil {
		t.Fatalf("UpdateOIDC: %v", err)
	}

	if _, err := database.GetSessionByID(t.Context(), res.SessionID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("session after disable: got %v, want ErrNotFound", err)
	}
}

func TestUseOIDCStateOnce(t *testing.T) {
	database := setupTestDB(t)
	hash := sha256.Sum256([]byte("state"))
	now := time.Now().Unix()

	if err := database.UseOIDCState(t.Context(), hash[:], now+600, now); err != nil {
		t.Fatalf("UseOIDCState: %v", err)
	}

	if err := database.UseOIDCState(t.Context(), hash[:], now+600, now); !errors.Is(err, db.ErrStateUsed) {
		t.Fatalf("replay: got %v, want ErrStateUsed", err)
	}

	if err := database.UseOIDCState(t.Context(), hash[:], now+1200, now+601); err != nil {
		t.Fatalf("after expiry: %v", err)
	}
}
