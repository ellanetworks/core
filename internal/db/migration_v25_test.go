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
