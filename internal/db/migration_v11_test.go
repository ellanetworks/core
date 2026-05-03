// Copyright 2026 Ella Networks

package db

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

// TestMigrationV11_PopulatedDB exercises the v11 data migration with
// representative pre-migration rows for every table that v11 retypes.
// It verifies:
//   - the new id is the deterministic UUIDv5 the spec requires;
//   - integer FK columns are rewritten to the parent's UUID, so
//     joins still resolve and PRAGMA foreign_key_check is clean;
//   - all rows survive the rebuild.
//
// This is the only end-to-end check that the migration is correct with
// real data. Without it, an emptied-DB-only test path would let a
// regression in `rebuildTableUUID` (e.g. wrong column order, dropped FK
// rewrite) ship undetected.
func TestMigrationV11_PopulatedDB(t *testing.T) {
	tmp := t.TempDir()

	conn, err := openSQLiteConnection(context.Background(), filepath.Join(tmp, "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	ctx := context.Background()

	// Bring schema to v10.
	if err := runMigrations(ctx, conn, 10); err != nil {
		t.Fatalf("migrate to v10: %v", err)
	}

	// Seed a complete dependency chain at v10:
	// data_network(id=42) ← policy(id=7, dataNetworkID=42, profileID=3, sliceID=5)
	//   ← network_rule(policy_id=7)
	//   ← ip_lease(poolID=42)
	// profile(id=3) ← subscriber(profileID=3)
	// network_slice(id=5)
	// user(id=11) ← session(user_id=11), api_token(user_id=11)
	stmts := []string{
		`INSERT INTO data_networks (id, name, ipPool, dns, mtu) VALUES (42, 'dn1', '10.0.0.0/24', '8.8.8.8', 1500)`,
		`INSERT INTO network_slices (id, sst, sd, name) VALUES (5, 1, '000001', 'slice-a')`,
		`INSERT INTO profiles (id, name, ueAmbrUplink, ueAmbrDownlink) VALUES (3, 'profile-a', '10 Mbps', '20 Mbps')`,
		`INSERT INTO policies (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink)
		   VALUES (7, 'policy-a', 3, 5, 42, 9, 1, '5 Mbps', '10 Mbps')`,
		`INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID)
		   VALUES (1, '001010000000001', '000000000001', 'aabbccddeeff00112233445566778899', '0011223344556677889900aabbccddee', 3)`,
		`INSERT INTO network_rules (id, policy_id, description, direction, protocol, port_low, port_high, action, precedence)
		   VALUES (100, 7, 'allow-http', 'uplink', 6, 80, 80, 'allow', 1)`,
		`INSERT INTO ip_leases (id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID)
		   VALUES (200, 42, X'00000000000000000000FFFF0A000001', '001010000000001', 1, 'dynamic', 1234567890, 0)`,
		`INSERT INTO users (id, email, roleID, hashedPassword) VALUES (11, 'admin@test', 1, 'hash')`,
		`INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at)
		   VALUES (300, 11, X'01020304', 1234567890, 9999999999)`,
		`INSERT INTO api_tokens (id, token_id, name, token_hash, user_id, expires_at)
		   VALUES (400, 'tok1', 'token-a', 'hash', 11, NULL)`,
		`INSERT INTO audit_logs (timestamp, level, actor, action, ip, details)
		   VALUES ('2026-01-01T00:00:00Z', 'info', 'admin', 'create', '127.0.0.1', '')`,
		`INSERT INTO home_network_keys (key_identifier, scheme, private_key) VALUES (1, 'A', 'deadbeef')`,
		`INSERT INTO retention_policies (category, retention_days) VALUES ('audit', 7)`,
	}

	for _, s := range stmts {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed v10 row %q: %v", s, err)
		}
	}

	// Run v11.
	if err := runMigrations(ctx, conn, 11); err != nil {
		t.Fatalf("migrate to v11: %v", err)
	}

	// Helper: deterministic UUID for (table, oldID) — must match the
	// scheme used by rebuildTableUUID.
	expectUUID := func(ns uuid.UUID, table string, oldID int64) string {
		return uuid.NewSHA1(ns, []byte(table+":"+strconv.FormatInt(oldID, 10))).String()
	}

	// Verify each parent's id and that its child's FK column resolves.
	cases := []struct {
		query    string
		wantUUID string
	}{
		{`SELECT id FROM data_networks WHERE name='dn1'`, expectUUID(v11NSDataNetworks, DataNetworksTableName, 42)},
		{`SELECT id FROM network_slices WHERE name='slice-a'`, expectUUID(v11NSNetworkSlices, NetworkSlicesTableName, 5)},
		{`SELECT id FROM profiles WHERE name='profile-a'`, expectUUID(v11NSProfiles, ProfilesTableName, 3)},
		{`SELECT id FROM policies WHERE name='policy-a'`, expectUUID(v11NSPolicies, PoliciesTableName, 7)},
		{`SELECT id FROM users WHERE email='admin@test'`, expectUUID(v11NSUsers, UsersTableName, 11)},
		{`SELECT id FROM subscribers WHERE imsi='001010000000001'`, expectUUID(v11NSSubscribers, SubscribersTableName, 1)},
		{`SELECT profileID FROM policies WHERE name='policy-a'`, expectUUID(v11NSProfiles, ProfilesTableName, 3)},
		{`SELECT sliceID FROM policies WHERE name='policy-a'`, expectUUID(v11NSNetworkSlices, NetworkSlicesTableName, 5)},
		{`SELECT dataNetworkID FROM policies WHERE name='policy-a'`, expectUUID(v11NSDataNetworks, DataNetworksTableName, 42)},
		{`SELECT profileID FROM subscribers WHERE imsi='001010000000001'`, expectUUID(v11NSProfiles, ProfilesTableName, 3)},
		{`SELECT policy_id FROM network_rules WHERE description='allow-http'`, expectUUID(v11NSPolicies, PoliciesTableName, 7)},
		{`SELECT poolID FROM ip_leases WHERE imsi='001010000000001'`, expectUUID(v11NSDataNetworks, DataNetworksTableName, 42)},
		{`SELECT user_id FROM sessions WHERE token_hash=X'01020304'`, expectUUID(v11NSUsers, UsersTableName, 11)},
		{`SELECT user_id FROM api_tokens WHERE token_id='tok1'`, expectUUID(v11NSUsers, UsersTableName, 11)},
	}

	for _, c := range cases {
		var got string
		if err := conn.QueryRowContext(ctx, c.query).Scan(&got); err != nil {
			t.Errorf("%s: scan: %v", c.query, err)
			continue
		}

		if got != c.wantUUID {
			t.Errorf("%s = %q, want %q", c.query, got, c.wantUUID)
		}
	}

	// FK integrity must hold post-migration.
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable FK: %v", err)
	}

	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			child  string
			rowid  int64
			parent string
			fkid   int64
		)

		if err := rows.Scan(&child, &rowid, &parent, &fkid); err != nil {
			t.Fatalf("scan fk row: %v", err)
		}

		t.Errorf("FK violation after v11: child=%s row=%d parent=%s fk=%d", child, rowid, parent, fkid)
	}
}
