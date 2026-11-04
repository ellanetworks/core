// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	hraft "github.com/hashicorp/raft"
)

var (
	staleConfiguration = hraft.Configuration{Servers: []hraft.Server{{ID: "9", Address: "10.9.9.9:7000"}}}
	thirdConfiguration = hraft.Configuration{Servers: []hraft.Server{
		{ID: "1", Address: "10.0.0.1:7000"},
		{ID: "2", Address: "10.0.0.2:7000"},
		{ID: "3", Address: "10.0.0.3:7000"},
	}}
)

func commandLog(t *testing.T, index, term uint64) *hraft.Log {
	t.Helper()

	cmd, err := NewCommand(CmdChangeset, map[string]uint64{"index": index})
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}

	return &hraft.Log{Index: index, Term: term, Type: hraft.LogCommand, Data: data}
}

func seedAppliedState(t *testing.T, a *testApplier, state AppliedState) {
	t.Helper()

	if err := overwriteAppliedState(context.Background(), a.PlainDB(), state); err != nil {
		t.Fatalf("seed fsm_state: %v", err)
	}
}

func assertAppliedState(t *testing.T, a *testApplier, index, term, configurationIndex uint64, servers int) {
	t.Helper()

	state := appliedState(t, a)
	if state.Index != index || state.Term != term || state.ConfigurationIndex != configurationIndex {
		t.Fatalf("fsm_state = {index %d, term %d, configuration index %d}, want {%d, %d, %d}",
			state.Index, state.Term, state.ConfigurationIndex, index, term, configurationIndex)
	}

	if got := len(hraft.DecodeConfiguration(state.Configuration).Servers); got != servers {
		t.Fatalf("fsm_state.configuration has %d servers, want %d", got, servers)
	}
}

func TestEnsureFSMStateSchemaUpgradesLegacyTable(t *testing.T) {
	ctx := context.Background()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE fsm_state (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			lastApplied INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		t.Fatalf("create legacy fsm_state: %v", err)
	}

	if _, err := db.ExecContext(ctx, "INSERT INTO fsm_state (id, lastApplied) VALUES (1, 42)"); err != nil {
		t.Fatalf("seed legacy fsm_state: %v", err)
	}

	for range 2 {
		if err := EnsureFSMStateSchema(ctx, db); err != nil {
			t.Fatalf("EnsureFSMStateSchema: %v", err)
		}
	}

	state, err := readAppliedState(ctx, db)
	if err != nil {
		t.Fatalf("read upgraded fsm_state: %v", err)
	}

	if state.Index != 42 || state.Term != 0 || state.Configuration != nil || state.ConfigurationIndex != 0 {
		t.Fatalf("upgraded fsm_state = %+v, want lastApplied 42 preserved and the new columns empty", state)
	}

	var rows int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fsm_state").Scan(&rows); err != nil {
		t.Fatalf("count fsm_state rows: %v", err)
	}

	if rows != 1 {
		t.Fatalf("fsm_state has %d rows, want 1", rows)
	}
}

func TestApplyBatchRecordsTermAndConfiguration(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	fsm.ApplyBatch([]*hraft.Log{
		configurationEntry(1, 1, legacyConfiguration),
		commandLog(t, 2, 1),
		{Index: 3, Term: 2, Type: hraft.LogNoop},
		commandLog(t, 4, 2),
	})

	assertAppliedState(t, a, 4, 2, 1, 1)

	fsm.ApplyBatch([]*hraft.Log{
		commandLog(t, 5, 2),
		configurationEntry(6, 3, grownConfiguration),
	})

	assertAppliedState(t, a, 6, 3, 6, 2)

	if got := fsm.AppliedIndex(); got != 6 {
		t.Fatalf("AppliedIndex = %d, want 6", got)
	}
}

func TestApplyBatchSkipsEntriesAlreadyApplied(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	seedAppliedState(t, a, AppliedState{
		Index:              5,
		Term:               2,
		Configuration:      hraft.EncodeConfiguration(grownConfiguration),
		ConfigurationIndex: 4,
	})

	fsm.ApplyBatch([]*hraft.Log{
		configurationEntry(3, 1, legacyConfiguration),
		commandLog(t, 4, 1),
		commandLog(t, 5, 2),
	})

	if n := len(a.seen()); n != 0 {
		t.Fatalf("replayed %d already-applied commands", n)
	}

	assertAppliedState(t, a, 5, 2, 4, 2)

	fsm.ApplyBatch([]*hraft.Log{
		commandLog(t, 5, 2),
		configurationEntry(6, 3, thirdConfiguration),
		commandLog(t, 7, 3),
	})

	if n := len(a.seen()); n != 1 {
		t.Fatalf("applied %d commands, want only index 7", n)
	}

	assertAppliedState(t, a, 7, 3, 6, 3)
}

func TestApplyBatchReplacesStaleConfigurationAfterReset(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	seedAppliedState(t, a, AppliedState{
		Configuration:      hraft.EncodeConfiguration(staleConfiguration),
		ConfigurationIndex: 50,
	})

	fsm.ApplyBatch([]*hraft.Log{
		configurationEntry(1, 1, legacyConfiguration),
		commandLog(t, 2, 1),
	})

	state := appliedState(t, a)
	if state.Index != 2 || state.Term != 1 || state.ConfigurationIndex != 1 {
		t.Fatalf("fsm_state = %+v, want index 2, term 1, configuration index 1", state)
	}

	servers := hraft.DecodeConfiguration(state.Configuration).Servers
	if len(servers) != 1 || servers[0].ID != legacyConfiguration.Servers[0].ID {
		t.Fatalf("fsm_state.configuration = %+v, want the freshly bootstrapped configuration", servers)
	}
}

func TestSnapshotSinkCloseNeverRewindsPosition(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	store, err := newDatabaseSnapshotStore(fsm, t.TempDir())
	if err != nil {
		t.Fatalf("snapshot store: %v", err)
	}

	fsm.ApplyBatch([]*hraft.Log{
		configurationEntry(1, 1, legacyConfiguration),
		commandLog(t, 2, 1),
	})

	sink, err := store.Create(hraft.SnapshotVersionMax, 2, 1, legacyConfiguration, 1, nil)
	if err != nil {
		t.Fatalf("create sink: %v", err)
	}

	fsm.ApplyBatch([]*hraft.Log{
		commandLog(t, 3, 2),
		configurationEntry(4, 2, grownConfiguration),
		commandLog(t, 5, 2),
	})

	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	assertAppliedState(t, a, 5, 2, 4, 2)

	metas, err := store.List()
	if err != nil || len(metas) != 1 {
		t.Fatalf("list: %d metas, %v", len(metas), err)
	}

	if metas[0].Index != 5 || metas[0].Term != 2 || metas[0].ConfigurationIndex != 4 {
		t.Fatalf("listed snapshot = %+v, want index 5, term 2, configuration index 4", metas[0])
	}
}

func TestInstallOldFormatSnapshotStampsPosition(t *testing.T) {
	ctx := context.Background()

	oldPath := filepath.Join(t.TempDir(), "old.db")

	old, err := sql.Open("sqlite3", oldPath)
	if err != nil {
		t.Fatalf("open old database: %v", err)
	}

	for _, stmt := range []string{
		"CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT NOT NULL)",
		"INSERT INTO t (id, v) VALUES (1, 'old'), (2, 'old'), (3, 'old')",
		"CREATE TABLE fsm_state (id INTEGER PRIMARY KEY CHECK(id = 1), lastApplied INTEGER NOT NULL DEFAULT 0)",
		"INSERT INTO fsm_state (id, lastApplied) VALUES (1, 7)",
	} {
		if _, err := old.ExecContext(ctx, stmt); err != nil {
			_ = old.Close()

			t.Fatalf("%s: %v", stmt, err)
		}
	}

	_ = old.Close()

	image, err := os.ReadFile(oldPath) // #nosec: G304 — test temp file
	if err != nil {
		t.Fatalf("read old database: %v", err)
	}

	var header [snapshotHeaderSize]byte

	writeSnapshotHeader(header[:], 1, 1)

	a, restores := countingApplier(t)
	fsm := NewFSM(a, t.TempDir())
	raftDir := t.TempDir()

	store, err := newDatabaseSnapshotStore(fsm, raftDir)
	if err != nil {
		t.Fatalf("snapshot store: %v", err)
	}

	sink, err := store.Create(hraft.SnapshotVersionMax, 20, 4, grownConfiguration, 3, nil)
	if err != nil {
		t.Fatalf("create sink: %v", err)
	}

	if _, err := io.Copy(sink, io.MultiReader(bytes.NewReader(header[:]), bytes.NewReader(image))); err != nil {
		t.Fatalf("write old-format stream: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	meta, rc, err := store.Open(sink.ID())
	if err != nil {
		t.Fatalf("open install: %v", err)
	}

	if meta.Index != 20 || meta.Term != 4 {
		t.Fatalf("install meta = %+v, want index 20, term 4", meta)
	}

	if err := fsm.Restore(rc); err != nil {
		t.Fatalf("restore old-format snapshot: %v", err)
	}

	if n := restores.Load(); n != 1 {
		t.Fatalf("restored %d times, want 1", n)
	}

	if n := rowCount(t, a); n != 3 {
		t.Fatalf("restored database has %d rows, want 3", n)
	}

	assertAppliedState(t, a, 20, 4, 3, 2)

	if got := fsm.AppliedIndex(); got != 20 {
		t.Fatalf("AppliedIndex = %d, want 20", got)
	}

	entries, err := os.ReadDir(filepath.Join(raftDir, snapshotInstallsDir))
	if err != nil {
		t.Fatalf("read install dir: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("install files left behind after restore: %d", len(entries))
	}

	fsm.ApplyBatch([]*hraft.Log{commandLog(t, 21, 4)})

	assertAppliedState(t, a, 21, 4, 3, 2)
}
