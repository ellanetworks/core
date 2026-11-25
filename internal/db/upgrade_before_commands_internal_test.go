// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

func writeDatabaseBeforeCommands(t *testing.T, lastApplied int) string {
	t.Helper()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "ella.db")

	conn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := runMigrations(ctx, conn, commandSchema-1); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	if err := ensureFsmStateTable(ctx, conn); err != nil {
		t.Fatalf("ensure fsm_state: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "UPDATE fsm_state SET lastApplied = ? WHERE id = 1", lastApplied); err != nil {
		t.Fatalf("set lastApplied: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	return dbPath
}

func TestClusterMemberFromBeforeCommandsRefusesToStart(t *testing.T) {
	dbPath := writeDatabaseBeforeCommands(t, 5)

	cfg := ellaraft.FastTestConfig()
	cfg.Enabled = true
	cfg.RaftID = "1"

	if _, err := NewDatabase(t.Context(), dbPath, cfg); !errors.Is(err, ErrUpgradeFromBeforeCommands) {
		t.Fatalf("NewDatabase: err = %v, want ErrUpgradeFromBeforeCommands", err)
	}
}

func TestClusterUpgradePathAllowsDatabasesThatNeverJoinedALog(t *testing.T) {
	for _, tc := range []struct {
		name        string
		schema      int
		lastApplied int
	}{
		{"fresh", 0, 0},
		{"restore bundle", commandSchema - 1, 0},
		{"after commands", commandSchema, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			conn, err := openSQLiteConnection(ctx, filepath.Join(t.TempDir(), "ella.db"))
			if err != nil {
				t.Fatalf("open: %v", err)
			}

			defer func() { _ = conn.Close() }()

			if tc.schema > 0 {
				if err := runMigrations(ctx, conn, tc.schema); err != nil {
					t.Fatalf("runMigrations: %v", err)
				}

				if err := ensureFsmStateTable(ctx, conn); err != nil {
					t.Fatalf("ensure fsm_state: %v", err)
				}

				if _, err := conn.ExecContext(ctx, "UPDATE fsm_state SET lastApplied = ? WHERE id = 1", tc.lastApplied); err != nil {
					t.Fatalf("set lastApplied: %v", err)
				}
			}

			if err := checkClusterUpgradePath(ctx, conn); err != nil {
				t.Fatalf("checkClusterUpgradePath: %v", err)
			}
		})
	}
}

func TestStandaloneFromBeforeCommandsMigratesOnStart(t *testing.T) {
	const appliedIndex = 5

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbPath := writeDatabaseBeforeCommands(t, appliedIndex)
	boltPath := filepath.Join(filepath.Dir(dbPath), "raft", "raft.db")

	if err := os.MkdirAll(filepath.Dir(boltPath), 0o700); err != nil {
		t.Fatalf("mkdir raft: %v", err)
	}

	cmd, err := ellaraft.NewCommand(ellaraft.CmdChangeset, &bytesPayload{})
	if err != nil {
		t.Fatalf("build changeset entry: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal changeset entry: %v", err)
	}

	store, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("open log store: %v", err)
	}

	nodeID, err := ellaraft.ResolveRaftID(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("resolve raft id: %v", err)
	}

	logs := []*raft.Log{{
		Index: 1,
		Term:  1,
		Type:  raft.LogConfiguration,
		Data: raft.EncodeConfiguration(raft.Configuration{Servers: []raft.Server{{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(nodeID),
			Address:  "127.0.0.1:1",
		}}}),
	}}

	for i := uint64(2); i <= appliedIndex; i++ {
		logs = append(logs, &raft.Log{Index: i, Term: 1, Type: raft.LogCommand, Data: data})
	}

	if err := store.StoreLogs(logs); err != nil {
		t.Fatalf("store log entries: %v", err)
	}

	if err := store.SetUint64([]byte("CurrentTerm"), 1); err != nil {
		t.Fatalf("store term: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close log store: %v", err)
	}

	database, err := NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	if err := database.WaitUntilReady(ctx); err != nil {
		_ = database.Close()
		t.Fatalf("database never became ready: %v", err)
	}

	applied, err := database.CurrentSchemaVersion(ctx)

	_ = database.Close()

	if err != nil || applied != SchemaVersion() {
		t.Fatalf("schema = %d (%v), want %d", applied, err, SchemaVersion())
	}

	reopened, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("reopen log store: %v", err)
	}

	defer func() { _ = reopened.Close() }()

	if first, err := reopened.FirstIndex(); err != nil || (first != 0 && first <= appliedIndex) {
		t.Fatalf("first log index = %d (%v): the changeset entry at %d must be compacted", first, err, appliedIndex)
	}
}

func TestEntriesFromBeforeCommandsHaltTheApply(t *testing.T) {
	database := newCommandTestDB(t)

	for _, cmdType := range []ellaraft.CommandType{ellaraft.CmdChangeset, ellaraft.CmdChangesetBatch, ellaraft.CmdDeleteExpiredSessions} {
		_, err := database.ApplyCommand(t.Context(), &ellaraft.Command{Type: cmdType, Payload: []byte(`{}`)}, 7, 1)
		if !errors.Is(err, errPreCommandEntry) {
			t.Fatalf("ApplyCommand(%s): err = %v, want errPreCommandEntry", cmdType, err)
		}
	}
}
