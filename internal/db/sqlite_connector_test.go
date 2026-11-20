// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"testing"
)

func assertTempStoreMemory(t *testing.T, name string, pool *sql.DB) {
	t.Helper()

	var mode int
	if err := pool.QueryRowContext(context.Background(), "PRAGMA temp_store").Scan(&mode); err != nil {
		t.Fatalf("%s: read temp_store: %v", name, err)
	}

	if mode != 2 {
		t.Fatalf("%s: temp_store = %d, want 2 (MEMORY)", name, mode)
	}
}

func TestSQLiteConnectionsKeepTempStoreInMemory(t *testing.T) {
	database := newAtomicTestDB(t)

	assertTempStoreMemory(t, "writer", database.writer())
	assertTempStoreMemory(t, "reader", database.reader())
	assertTempStoreMemory(t, "plain", database.PlainDB())
}
