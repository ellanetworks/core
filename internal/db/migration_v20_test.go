// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationV20RetypesNodeIdentityAndSeedsPointers(t *testing.T) {
	tmp := t.TempDir()

	conn, err := openSQLiteConnection(context.Background(), filepath.Join(tmp, "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	ctx := context.Background()

	if err := runMigrations(ctx, conn, 19); err != nil {
		t.Fatalf("migrate to v19: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	seed := []string{
		`INSERT INTO cluster_members (nodeID, raftAddress, apiAddress, binaryVersion, suffrage, drainState, drainUpdatedAt)
		   VALUES (1, '10.0.0.1:7000', 'https://10.0.0.1:5000', 'v1.18.0', 'voter', 'active', 0)`,
		`INSERT INTO cluster_members (nodeID, raftAddress, apiAddress, binaryVersion, suffrage, drainState, drainUpdatedAt)
		   VALUES (3, '10.0.0.3:7000', 'https://10.0.0.3:5000', 'v1.18.0', 'nonvoter', 'drained', 42)`,
		`INSERT INTO cluster_node_certs (nodeID, fingerprint, certPEM, addedAt)
		   VALUES (1, 'sha256:aa', 'PEM-1', 100)`,
		`INSERT INTO cluster_join_tokens (id, nodeID, claimsJSON, expiresAt, consumedAt, consumedBy)
		   VALUES ('tok-open', 2, '{}', 900, 0, 0)`,
		`INSERT INTO cluster_join_tokens (id, nodeID, claimsJSON, expiresAt, consumedAt, consumedBy)
		   VALUES ('tok-used', 3, '{}', 900, 500, 3)`,
	}

	for _, stmt := range seed {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := runMigrations(ctx, conn, 20); err != nil {
		t.Fatalf("migrate to v20: %v", err)
	}

	for _, tc := range []struct {
		nodeID      string
		wantPointer int
		wantDrain   string
	}{
		{"1", 1, "active"},
		{"3", 3, "drained"},
	} {
		var (
			pointer int
			drain   string
			name    string
		)

		row := conn.QueryRowContext(ctx,
			"SELECT amfPointer, drainState, displayName FROM cluster_members WHERE nodeID = ?", tc.nodeID)
		if err := row.Scan(&pointer, &drain, &name); err != nil {
			t.Fatalf("read member %s: %v", tc.nodeID, err)
		}

		if pointer != tc.wantPointer {
			t.Errorf("member %s: amfPointer = %d, want %d (the integer it already had)", tc.nodeID, pointer, tc.wantPointer)
		}

		if drain != tc.wantDrain {
			t.Errorf("member %s: drainState = %q, want %q", tc.nodeID, drain, tc.wantDrain)
		}

		if name != "" {
			t.Errorf("member %s: displayName = %q, want empty", tc.nodeID, name)
		}
	}

	var certNodeID string
	if err := conn.QueryRowContext(ctx,
		"SELECT nodeID FROM cluster_node_certs WHERE fingerprint = 'sha256:aa'").Scan(&certNodeID); err != nil {
		t.Fatalf("read pin: %v", err)
	}

	if certNodeID != "1" {
		t.Errorf("pin owner = %q, want 1", certNodeID)
	}

	if _, err := conn.ExecContext(ctx,
		`INSERT INTO cluster_node_certs (nodeID, fingerprint, certPEM, addedAt)
		   VALUES ('0199c0de-0000-7000-8000-00000000beef', 'sha256:bb', 'PEM-UUID', 200)`); err != nil {
		t.Fatalf("pin a UUID identity: %v", err)
	}

	for _, tc := range []struct {
		id             string
		wantConsumedBy string
	}{
		{"tok-open", ""},
		{"tok-used", "3"},
	} {
		var consumedBy string

		if err := conn.QueryRowContext(ctx,
			"SELECT consumedBy FROM cluster_join_tokens WHERE id = ?", tc.id).Scan(&consumedBy); err != nil {
			t.Fatalf("read token %s: %v", tc.id, err)
		}

		if consumedBy != tc.wantConsumedBy {
			t.Errorf("token %s: consumedBy = %q, want %q", tc.id, consumedBy, tc.wantConsumedBy)
		}
	}

	var tokenColumns int
	if err := conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('cluster_join_tokens') WHERE name = 'nodeID'").Scan(&tokenColumns); err != nil {
		t.Fatalf("inspect cluster_join_tokens: %v", err)
	}

	if tokenColumns != 0 {
		t.Error("cluster_join_tokens.nodeID must be gone: a join token names no node")
	}

	var leaseNodeType string
	if err := conn.QueryRowContext(ctx,
		"SELECT type FROM pragma_table_info('ip_leases') WHERE name = 'nodeID'").Scan(&leaseNodeType); err != nil {
		t.Fatalf("inspect ip_leases: %v", err)
	}

	if leaseNodeType != "TEXT" {
		t.Errorf("ip_leases.nodeID type = %q, want TEXT", leaseNodeType)
	}
}
