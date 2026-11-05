// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

type fsmPosition struct {
	index              uint64
	term               uint64
	configuration      []byte
	configurationIndex uint64
}

func readFSMPosition(t *testing.T, database *db.Database) fsmPosition {
	t.Helper()

	var p fsmPosition

	if err := database.PlainDB().QueryRowContext(context.Background(),
		"SELECT lastApplied, lastAppliedTerm, configuration, configurationIndex FROM fsm_state WHERE id = 1",
	).Scan(&p.index, &p.term, &p.configuration, &p.configurationIndex); err != nil {
		t.Fatalf("read fsm_state: %v", err)
	}

	return p
}

func openReadyDatabase(t *testing.T, path string) *db.Database {
	t.Helper()

	database, err := db.NewDatabase(context.Background(), path, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		_ = database.Close()

		t.Fatalf("database never became ready: %v", err)
	}

	return database
}

func TestRestoreStampsRaftPositionAndSurvivesRestart(t *testing.T) {
	tempDir := t.TempDir()
	databasePath := filepath.Join(tempDir, "data")
	ctx := context.Background()

	database := openReadyDatabase(t, databasePath)

	const imsi = "001010000000077"

	if _, err := createDataNetworkPolicyAndSubscriber(database, imsi); err != nil {
		_ = database.Close()

		t.Fatalf("seed subscriber: %v", err)
	}

	backupFile, err := os.CreateTemp(tempDir, "backup_*.tar.gz")
	if err != nil {
		_ = database.Close()

		t.Fatalf("create backup file: %v", err)
	}

	defer func() { _ = backupFile.Close() }()

	if err := database.Backup(ctx, backupFile); err != nil {
		_ = database.Close()

		t.Fatalf("backup: %v", err)
	}

	if err := database.DeleteSubscriber(ctx, imsi); err != nil {
		_ = database.Close()

		t.Fatalf("delete subscriber: %v", err)
	}

	before := readFSMPosition(t, database)

	if err := database.Restore(ctx, backupFile); err != nil {
		_ = database.Close()

		t.Fatalf("restore: %v", err)
	}

	restored := readFSMPosition(t, database)

	if restored.index <= before.index || restored.term == 0 || restored.term < before.term {
		_ = database.Close()

		t.Fatalf("restore stamped position %d at term %d, want past %d at term >= %d",
			restored.index, restored.term, before.index, before.term)
	}

	if len(restored.configuration) == 0 || restored.configurationIndex == 0 {
		_ = database.Close()

		t.Fatalf("restore must stamp the raft configuration: %+v", restored)
	}

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: "after-restore", IPv4Pool: "10.45.0.0/24"}); err != nil {
		_ = database.Close()

		t.Fatalf("write after restore: %v", err)
	}

	afterWrite := readFSMPosition(t, database)

	if afterWrite.index <= restored.index {
		_ = database.Close()

		t.Fatalf("a write after the restore did not advance fsm_state: %d <= %d", afterWrite.index, restored.index)
	}

	if err := database.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	database = openReadyDatabase(t, databasePath)

	defer func() { _ = database.Close() }()

	if got := readFSMPosition(t, database); got.index < afterWrite.index {
		t.Fatalf("restart rewound fsm_state.lastApplied from %d to %d", afterWrite.index, got.index)
	}

	subs, total, err := database.ListSubscribersPage(ctx, nil, 1, 10)
	if err != nil {
		t.Fatalf("list subscribers after restart: %v", err)
	}

	if total != 1 || len(subs) != 1 || subs[0].Imsi != imsi {
		t.Fatalf("after restart: subscribers %+v, want the restored %s", subs, imsi)
	}

	if _, err := database.GetDataNetwork(ctx, "after-restore"); err != nil {
		t.Fatalf("a write made after the restore was lost across restart: %v", err)
	}

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: "after-restart", IPv4Pool: "10.46.0.0/24"}); err != nil {
		t.Fatalf("write after restart: %v", err)
	}

	if got := readFSMPosition(t, database); got.index <= afterWrite.index {
		t.Fatalf("a write after restart did not advance fsm_state: %d <= %d", got.index, afterWrite.index)
	}
}
