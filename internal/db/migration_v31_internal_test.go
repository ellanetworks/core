// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"strings"
	"testing"
)

func seedOrphanSubscriber(t *testing.T, database *Database) {
	t.Helper()

	ctx := t.Context()

	createTestSubscriber(t, database, "001010000000901")

	conn, err := database.writer().Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	defer func() { _ = conn.Close() }()

	for _, stmt := range []string{
		"PRAGMA foreign_keys = OFF",
		"DELETE FROM profiles WHERE id = (SELECT profileID FROM subscribers WHERE imsi = '001010000000901')",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

func TestMigrationV31KeepsGoingWhenRowsBreakAForeignKey(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := t.Context()

	seedOrphanSubscriber(t, database)

	violations, err := foreignKeyViolationsOf(ctx, database.writer())
	if err != nil {
		t.Fatalf("foreignKeyViolationsOf: %v", err)
	}

	if len(violations) != 1 || !strings.Contains(violations[0], "subscribers") {
		t.Fatalf("violations = %v, want the orphaned subscriber", violations)
	}

	for _, stmt := range []string{"DROP TABLE " + SubscriberCountTableName, "DROP INDEX idx_leases_pool_latest"} {
		if _, err := database.writer().ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	tx, err := database.writer().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	if err := migrateV31(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV31 must not fail on rows that break a foreign key: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestStandaloneNodesDoNotHoldTheCommandSchemaForForeignKeys(t *testing.T) {
	database := newStandaloneDB(t)

	seedOrphanSubscriber(t, database)

	blocked, _, err := database.commandSchemaBlocked(t.Context(), commandSchema-1, commandSchema)
	if err != nil {
		t.Fatalf("commandSchemaBlocked: %v", err)
	}

	if blocked {
		t.Fatal("a standalone node has no other node to diverge from and must not hold the command schema")
	}
}
