// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationV23SetsIMSSignalling5QIOnIMSPolicies(t *testing.T) {
	ctx := context.Background()

	conn, err := openSQLiteConnection(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	if err := runMigrations(ctx, conn, 22); err != nil {
		t.Fatalf("migrate to v22: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	seed := []string{
		`INSERT INTO data_networks (id, name, ipPool, ipv6Pool, dns, mtu) VALUES ('dn-ims', 'ims', '10.60.0.0/16', '', '8.8.8.8', 1400)`,
		`INSERT INTO data_networks (id, name, ipPool, ipv6Pool, dns, mtu) VALUES ('dn-internet', 'internet', '10.45.0.0/16', '', '8.8.8.8', 1400)`,
		`INSERT INTO policies (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink)
		   VALUES ('p1', 'ims-9', 'pr', 'sl', 'dn-ims', 9, 15, '100 Mbps', '100 Mbps')`,
		`INSERT INTO policies (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink)
		   VALUES ('p2', 'ims-5', 'pr2', 'sl', 'dn-ims', 5, 1, '100 Mbps', '100 Mbps')`,
		`INSERT INTO policies (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink)
		   VALUES ('p3', 'internet-9', 'pr', 'sl', 'dn-internet', 9, 15, '100 Mbps', '100 Mbps')`,
	}

	for _, stmt := range seed {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := runMigrations(ctx, conn, 23); err != nil {
		t.Fatalf("migrate to v23: %v", err)
	}

	for name, want := range map[string]struct{ fiveQI, arp int }{"ims-9": {5, 15}, "ims-5": {5, 1}, "internet-9": {9, 15}} {
		var fiveQI, arp int

		if err := conn.QueryRowContext(ctx, "SELECT var5qi, arp FROM policies WHERE name = ?", name).Scan(&fiveQI, &arp); err != nil {
			t.Fatalf("read policy %s: %v", name, err)
		}

		if fiveQI != want.fiveQI || arp != want.arp {
			t.Errorf("policy %s: 5QI %d, ARP %d, want 5QI %d, ARP %d", name, fiveQI, arp, want.fiveQI, want.arp)
		}
	}
}
