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

func TestOpenSQLiteConnectionAppliesPragmasToEveryConnection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ella.db")

	conn, err := openSQLiteConnection(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	defer func() { _ = conn.Close() }()

	want := map[string]string{
		"journal_mode": "wal",
		"busy_timeout": "5000",
		"synchronous":  "1",
		"foreign_keys": "1",
	}

	check := func(stage string) {
		t.Helper()

		for pragma, expected := range want {
			var got string
			if err := conn.QueryRowContext(context.Background(), "PRAGMA "+pragma).Scan(&got); err != nil {
				t.Fatalf("%s: read %s: %v", stage, pragma, err)
			}

			if got != expected {
				t.Fatalf("%s: %s = %q, want %q", stage, pragma, got, expected)
			}
		}
	}

	check("initial connection")

	conn.SetMaxIdleConns(0)
	conn.SetMaxIdleConns(1)

	check("replacement connection")
}
