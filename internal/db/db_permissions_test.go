// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteConnectionCreatesPrivateFiles(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ella.db")

	conn, err := openSQLiteConnection(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(context.Background(), "CREATE TABLE t (x INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	for _, suffix := range []string{"", "-wal", "-shm"} {
		path := dbPath + suffix

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}

		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("%s has mode %v, want no group or world bits", path, perm)
		}
	}
}
