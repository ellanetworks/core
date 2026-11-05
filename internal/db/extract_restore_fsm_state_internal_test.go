// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func backupBundleWithFSMState(t *testing.T, statements ...string) string {
	t.Helper()

	ctx := context.Background()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.db")

	source, err := sql.Open("sqlite3", sourcePath)
	if err != nil {
		t.Fatalf("open source database: %v", err)
	}

	for _, stmt := range statements {
		if _, err := source.ExecContext(ctx, stmt); err != nil {
			_ = source.Close()

			t.Fatalf("%s: %v", stmt, err)
		}
	}

	if err := source.Close(); err != nil {
		t.Fatalf("close source database: %v", err)
	}

	image, err := os.ReadFile(sourcePath) // #nosec: G304 — test temp file
	if err != nil {
		t.Fatalf("read source database: %v", err)
	}

	bundlePath := filepath.Join(dir, "bundle.tar.gz")

	archive := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion)},
		{name: DBFilename, data: image},
	})

	if err := os.WriteFile(bundlePath, archive, 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	return bundlePath
}

func assertResetFSMState(t *testing.T, dbPath string) {
	t.Helper()

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open extracted database: %v", err)
	}

	defer func() { _ = conn.Close() }()

	var (
		lastApplied, lastAppliedTerm, configurationIndex int64
		configuration                                    []byte
	)

	if err := conn.QueryRowContext(context.Background(),
		"SELECT lastApplied, lastAppliedTerm, configuration, configurationIndex FROM fsm_state WHERE id = 1",
	).Scan(&lastApplied, &lastAppliedTerm, &configuration, &configurationIndex); err != nil {
		t.Fatalf("read extracted fsm_state: %v", err)
	}

	if lastApplied != 0 || lastAppliedTerm != 0 || configuration != nil || configurationIndex != 0 {
		t.Fatalf("extracted fsm_state = {%d, %d, %v, %d}, want every raft position column reset",
			lastApplied, lastAppliedTerm, configuration, configurationIndex)
	}
}

func TestExtractForRestoreResetsRaftPosition(t *testing.T) {
	bundle := backupBundleWithFSMState(t,
		`CREATE TABLE fsm_state (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			lastApplied INTEGER NOT NULL DEFAULT 0,
			lastAppliedTerm INTEGER NOT NULL DEFAULT 0,
			configuration BLOB,
			configurationIndex INTEGER NOT NULL DEFAULT 0
		)`,
		"INSERT INTO fsm_state VALUES (1, 120, 7, x'0102', 33)",
	)

	dbPath := filepath.Join(t.TempDir(), DBFilename)

	if err := ExtractForRestore(bundle, dbPath); err != nil {
		t.Fatalf("ExtractForRestore: %v", err)
	}

	assertResetFSMState(t, dbPath)
}

func TestExtractForRestoreUpgradesLegacyFSMState(t *testing.T) {
	bundle := backupBundleWithFSMState(t,
		"CREATE TABLE fsm_state (id INTEGER PRIMARY KEY CHECK(id = 1), lastApplied INTEGER NOT NULL DEFAULT 0)",
		"INSERT INTO fsm_state VALUES (1, 120)",
	)

	dbPath := filepath.Join(t.TempDir(), DBFilename)

	if err := ExtractForRestore(bundle, dbPath); err != nil {
		t.Fatalf("ExtractForRestore of a pre-upgrade archive: %v", err)
	}

	assertResetFSMState(t, dbPath)
}
