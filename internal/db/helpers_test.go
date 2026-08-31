// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func setupTestDB(t *testing.T) *db.Database {
	t.Helper()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	t.Cleanup(func() {
		if err := dbInstance.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	})

	return dbInstance
}
