// Copyright 2026 Ella Networks

package db

import (
	"context"
	"fmt"
	"net/netip"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateV6_BackfillsAddressBin(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Build schema up to V5 (before addressBin exists).
	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
	applyV5(t, db)

	// Insert prerequisite data: operator, data network, policy, subscriber.
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst) VALUES (1, '001', '01', 'testop', 1)",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, ipPool, dns, mtu) VALUES (1, 'test-dnn', '10.0.0.0/24', '8.8.8.8', 1400)",
		DataNetworksTableName))
	if err != nil {
		t.Fatalf("failed to insert data network: %v", err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID) VALUES (1, 'test-policy', '1 Mbps', '1 Mbps', 9, 1, 1)",
		PoliciesTableName))
	if err != nil {
		t.Fatalf("failed to insert policy: %v", err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (imsi, sequenceNumber, permanentKey, opc, policyID) VALUES ('001010100000001', '000000000001', '6f30087629feb0b089783c81d0ae09b5', '21a7e1897dfb481d62439142cdf1b6ee', 1)",
		SubscribersTableName))
	if err != nil {
		t.Fatalf("failed to insert subscriber: %v", err)
	}

	// Insert TEXT-only leases (no addressBin column yet).
	leases := []struct {
		address string
		session int
	}{
		{"10.0.0.1", 1},
		{"10.0.0.2", 2},
		{"10.0.0.10", 3},
	}

	for _, l := range leases {
		_, err := db.ExecContext(ctx, fmt.Sprintf(
			"INSERT INTO %s (poolID, address, imsi, sessionID, type, createdAt) VALUES (1, '%s', '001010100000001', %d, 'dynamic', 1000)",
			IPLeasesTableName, l.address, l.session))
		if err != nil {
			t.Fatalf("failed to insert lease %s: %v", l.address, err)
		}
	}

	// Apply V6 migration — this should backfill addressBin.
	applyV6(t, db)

	// Verify each row has the correct addressBin.
	for _, l := range leases {
		var binData []byte

		err := db.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT addressBin FROM %s WHERE address = ?", IPLeasesTableName), l.address).Scan(&binData)
		if err != nil {
			t.Fatalf("failed to query addressBin for %s: %v", l.address, err)
		}

		expected := netip.MustParseAddr(l.address).As16()

		if len(binData) != 16 {
			t.Errorf("address %s: expected 16 bytes, got %d", l.address, len(binData))
			continue
		}

		for i := 0; i < 16; i++ {
			if binData[i] != expected[i] {
				t.Errorf("address %s: byte %d = %02x, want %02x", l.address, i, binData[i], expected[i])
				break
			}
		}
	}

	// Verify ORDER BY addressBin produces correct numeric order.
	rows, err := db.QueryContext(ctx, fmt.Sprintf(
		"SELECT address FROM %s WHERE poolID = 1 ORDER BY addressBin", IPLeasesTableName))
	if err != nil {
		t.Fatalf("failed to query sorted addresses: %v", err)
	}

	defer func() { _ = rows.Close() }()

	var sorted []string

	for rows.Next() {
		var addr string

		if err := rows.Scan(&addr); err != nil {
			t.Fatalf("failed to scan address: %v", err)
		}

		sorted = append(sorted, addr)
	}

	expected := []string{"10.0.0.1", "10.0.0.2", "10.0.0.10"}

	if len(sorted) != len(expected) {
		t.Fatalf("expected %d addresses, got %d", len(expected), len(sorted))
	}

	for i, want := range expected {
		if sorted[i] != want {
			t.Fatalf("sorted[%d] = %s, want %s", i, sorted[i], want)
		}
	}
}

func TestMigrateV6_CreatesIndex(t *testing.T) {
	db := openTestDB(t)

	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
	applyV5(t, db)
	applyV6(t, db)

	indexes := allIndexNames(t, db)

	indexSet := make(map[string]bool, len(indexes))
	for _, name := range indexes {
		indexSet[name] = true
	}

	if !indexSet["idx_leases_pool_address_bin"] {
		t.Errorf("expected index idx_leases_pool_address_bin not found; got indexes: %v", indexes)
	}
}

func TestMigrateV6_AddsColumn(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
	applyV5(t, db)
	applyV6(t, db)

	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", IPLeasesTableName))
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}

	defer func() { _ = rows.Close() }()

	found := false

	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      interface{}
			pk        int
		)

		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}

		if name == "addressBin" {
			found = true

			if typ != "BLOB" {
				t.Errorf("addressBin type = %s, want BLOB", typ)
			}

			if notnull != 1 {
				t.Errorf("addressBin notnull = %d, want 1", notnull)
			}
		}
	}

	if !found {
		t.Error("addressBin column not found in ip_leases table after V6 migration")
	}
}
