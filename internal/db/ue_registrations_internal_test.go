// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestResetUERegistrationsPurgedInRestoredDB(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "restored.db")

	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %s", err)
	}

	defer func() { _ = conn.Close() }()

	stmts := []string{
		"CREATE TABLE ue_registrations (imsi TEXT NOT NULL, type TEXT NOT NULL, nodeID TEXT NOT NULL, purged INTEGER NOT NULL DEFAULT 0, version INTEGER NOT NULL, PRIMARY KEY (imsi, type))",
		"INSERT INTO ue_registrations VALUES ('001010000000001', 'mme', '1', 1, 1)",
		"INSERT INTO ue_registrations VALUES ('001010000000001', 'amf-3gpp-access', '2', 0, 2)",
		"INSERT INTO ue_registrations VALUES ('001010000000002', 'amf-3gpp-access', '1', 1, 3)",
	}

	for _, stmt := range stmts {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %s", stmt, err)
		}
	}

	if err := resetUERegistrationsInRestoredDB(ctx, path); err != nil {
		t.Fatalf("resetUERegistrationsInRestoredDB: %s", err)
	}

	var purged int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM ue_registrations WHERE purged!=0").Scan(&purged); err != nil {
		t.Fatalf("count purged: %s", err)
	}

	if purged != 0 {
		t.Fatalf("expected no purged registrations, got %d", purged)
	}

	var versioned int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM ue_registrations WHERE version!=0").Scan(&versioned); err != nil {
		t.Fatalf("count versioned: %s", err)
	}

	if versioned != 0 {
		t.Fatalf("expected every version reset, got %d versioned registrations", versioned)
	}
}

func TestResetUERegistrationsPurgedInRestoredDB_PreV21Backup(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "restored.db")

	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %s", err)
	}

	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "CREATE TABLE subscribers (imsi TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create: %s", err)
	}

	if err := resetUERegistrationsInRestoredDB(ctx, path); err != nil {
		t.Fatalf("resetUERegistrationsInRestoredDB: %s", err)
	}
}

func TestRegisterUE_RewritesRestoredRegistration(t *testing.T) {
	ctx := context.Background()

	database, err := NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	defer func() { _ = database.Close() }()

	profile := &Profile{Name: "profile", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}
	if err := database.CreateProfile(ctx, profile); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	created, err := database.GetProfile(ctx, profile.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	if err := database.CreateSubscriber(ctx, &Subscriber{
		Imsi:           "001010000000001",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      created.ID,
	}); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	if _, err := database.RegisterUE(ctx, "001010000000001", UERegistrationTypeMME, "node-a", ""); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if _, err := database.conn().PlainDB().ExecContext(ctx, "UPDATE ue_registrations SET version=0"); err != nil {
		t.Fatalf("zero versions: %s", err)
	}

	version, err := database.RegisterUE(ctx, "001010000000001", UERegistrationTypeMME, "node-a", "")
	if err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	reg, err := database.GetUERegistration(ctx, "001010000000001", UERegistrationTypeMME)
	if err != nil {
		t.Fatalf("GetUERegistration: %s", err)
	}

	if version == 0 || reg.Version != version {
		t.Fatalf("restored registration kept version %d (returned %d), want a new version", reg.Version, version)
	}
}
