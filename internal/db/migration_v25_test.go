// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationV25MarksIMSDataNetworkAndDefaultsPreemption(t *testing.T) {
	ctx := context.Background()

	conn, err := openSQLiteConnection(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	if err := runMigrations(ctx, conn, 24); err != nil {
		t.Fatalf("migrate to v24: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	seed := []string{
		`INSERT INTO data_networks (id, name, ipPool, ipv6Pool, dns, mtu) VALUES ('dn-ims', 'ims', '10.60.0.0/16', '', '8.8.8.8', 1400)`,
		`INSERT INTO data_networks (id, name, ipPool, ipv6Pool, dns, mtu) VALUES ('dn-internet', 'internet', '10.45.0.0/16', '', '8.8.8.8', 1400)`,
		`INSERT INTO policies (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink)
		   VALUES ('p1', 'ims-5', 'pr', 'sl', 'dn-ims', 5, 1, '100 Mbps', '100 Mbps')`,
	}

	for _, stmt := range seed {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := runMigrations(ctx, conn, 25); err != nil {
		t.Fatalf("migrate to v25: %v", err)
	}

	for name, want := range map[string]bool{"ims": true, "internet": false} {
		var ims bool

		if err := conn.QueryRowContext(ctx, "SELECT ims FROM data_networks WHERE name = ?", name).Scan(&ims); err != nil {
			t.Fatalf("read data network %s: %v", name, err)
		}

		if ims != want {
			t.Errorf("data network %s: ims %t, want %t", name, ims, want)
		}
	}

	var capability, vulnerability string

	if err := conn.QueryRowContext(ctx, "SELECT preemptionCapability, preemptionVulnerability FROM policies WHERE name = 'ims-5'").Scan(&capability, &vulnerability); err != nil {
		t.Fatalf("read policy: %v", err)
	}

	if capability != "NOT_PREEMPT" || vulnerability != "NOT_PREEMPTABLE" {
		t.Errorf("policy pre-emption %s/%s, want NOT_PREEMPT/NOT_PREEMPTABLE", capability, vulnerability)
	}

	if _, err := conn.ExecContext(ctx, "UPDATE policies SET preemptionCapability = 'SHALL_NOT_TRIGGER' WHERE name = 'ims-5'"); err == nil {
		t.Error("an unknown pre-emption capability was accepted")
	}
}

func TestMigrationV25SplitsIMSRegistrationsPerRegistrationSet(t *testing.T) {
	ctx := context.Background()

	conn, err := openSQLiteConnection(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	if err := runMigrations(ctx, conn, 24); err != nil {
		t.Fatalf("migrate to v24: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO ims_registrations (imsi, state, serverName, authPending, originHost, originRealm)
		VALUES ('001010000000001', 1, 'sip:scscf.example.org', 0, 'scscf.example.org', 'example.org')`); err != nil {
		t.Fatalf("seed an IMS registration: %v", err)
	}

	if err := runMigrations(ctx, conn, 25); err != nil {
		t.Fatalf("migrate to v25: %v", err)
	}

	for _, table := range []string{IMSRegistrationsTableName, IMSRegistrationSetsTableName} {
		var n int

		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s holds %d rows, %v; want the v24 registrations dropped", table, n, err)
		}
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	stmts := []string{
		`INSERT INTO ims_registrations (imsi, serverName, originHost, originRealm, privateIdentity)
		   VALUES ('001010000000001', 'sip:scscf.example.org', 'scscf.example.org', 'example.org', '001010000000001@example.org')`,
		`INSERT INTO ims_registration_sets (imsi, registrationSet, state, authPending, defaultIdentity, profileDigest, emergency)
		   VALUES ('001010000000001', 'mmtel', 1, 0, 'sip:+15550001@example.org', 'digest', 0)`,
		`INSERT INTO ims_registration_sets (imsi, registrationSet, state, authPending, defaultIdentity, profileDigest, emergency)
		   VALUES ('001010000000001', 'mcptt', 0, 1, '', '', 0)`,
	}

	for _, stmt := range stmts {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%q: %v", stmt, err)
		}
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO ims_registration_sets (imsi, registrationSet, state, authPending, defaultIdentity, profileDigest, emergency)
		VALUES ('001010000000001', 'rcs', 0, 1, '', '', 0)`); err == nil {
		t.Error("an unknown registration set was accepted")
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO ims_service_profiles (id, name) VALUES ('sp', 'volte')`); err != nil {
		t.Fatalf("insert a service profile: %v", err)
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO ims_service_profile_assignments (profileID, registrationSet, serviceProfileID) VALUES ('pr', 'mmtel', 'sp')`); err != nil {
		t.Fatalf("assign the service profile to the mmtel set: %v", err)
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO ims_service_profile_assignments (profileID, registrationSet, serviceProfileID) VALUES ('pr', 'rcs', 'sp')`); err == nil {
		t.Error("a service profile was assigned to an unknown registration set")
	}
}
