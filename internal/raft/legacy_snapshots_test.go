// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	hraft "github.com/hashicorp/raft"
)

var (
	legacyConfiguration = hraft.Configuration{Servers: []hraft.Server{{ID: "1", Address: "127.0.0.1:1"}}}
	grownConfiguration  = hraft.Configuration{Servers: []hraft.Server{
		{ID: "1", Address: "10.0.0.1:7000"},
		{ID: "2", Address: "10.0.0.2:7000"},
	}}
)

type legacyStore struct {
	raftDir  string
	fsm      *FSM
	applier  *testApplier
	restores *atomic.Int32
	logs     *hraft.InmemStore
}

func newLegacyStore(t *testing.T, lastApplied uint64) *legacyStore {
	t.Helper()

	a, restores := countingApplier(t)
	fsm := NewFSM(a, t.TempDir())

	if err := fsm.writeLastApplied(lastApplied); err != nil {
		t.Fatalf("seed lastApplied: %v", err)
	}

	return &legacyStore{
		raftDir:  t.TempDir(),
		fsm:      fsm,
		applier:  a,
		restores: restores,
		logs:     hraft.NewInmemStore(),
	}
}

func (s *legacyStore) log(t *testing.T, entries ...*hraft.Log) {
	t.Helper()

	if err := s.logs.StoreLogs(entries); err != nil {
		t.Fatalf("store legacy log entries: %v", err)
	}
}

func (s *legacyStore) snapshot(t *testing.T, index, term uint64, payload []byte) {
	t.Helper()

	store, err := hraft.NewFileSnapshotStore(s.raftDir, 3, os.Stderr)
	if err != nil {
		t.Fatalf("legacy snapshot store: %v", err)
	}

	_, transport := hraft.NewInmemTransport("")

	sink, err := store.Create(hraft.SnapshotVersionMax, index, term, legacyConfiguration, 1, transport)
	if err != nil {
		t.Fatalf("create legacy snapshot: %v", err)
	}

	if _, err := sink.Write(payload); err != nil {
		_ = sink.Cancel()

		t.Fatalf("write legacy snapshot: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close legacy snapshot: %v", err)
	}
}

func (s *legacyStore) migrate(t *testing.T) (bool, error) {
	t.Helper()

	return migrateLegacySnapshots(s.raftDir, hraft.DefaultConfig(), s.fsm, s.logs)
}

func legacyPayload(t *testing.T, rows int) []byte {
	t.Helper()

	a := newTestApplier(t)

	for i := 1; i <= rows; i++ {
		if _, err := a.db.ExecContext(context.Background(), "INSERT INTO t(id, v) VALUES (?, 'legacy')", i); err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}
	}

	fsm := NewFSM(a, t.TempDir())

	if err := fsm.writeLastApplied(1); err != nil {
		t.Fatalf("seed legacy lastApplied: %v", err)
	}

	return persistSnapshot(t, fsm)
}

func command(index, term uint64) *hraft.Log {
	return &hraft.Log{Index: index, Term: term, Type: hraft.LogCommand, Data: []byte("{}")}
}

func configurationEntry(index, term uint64, configuration hraft.Configuration) *hraft.Log {
	return &hraft.Log{Index: index, Term: term, Type: hraft.LogConfiguration, Data: hraft.EncodeConfiguration(configuration)}
}

func TestLegacyMigrationSkipsRestoreAcrossNonCommandGap(t *testing.T) {
	s := newLegacyStore(t, 5)
	s.log(t,
		command(5, 1),
		&hraft.Log{Index: 6, Term: 2, Type: hraft.LogNoop},
		&hraft.Log{Index: 7, Term: 2, Type: hraft.LogBarrier},
	)
	s.snapshot(t, 7, 2, legacyPayload(t, 9))

	migrated, err := s.migrate(t)
	if err != nil || !migrated {
		t.Fatalf("migrate: migrated=%v err=%v", migrated, err)
	}

	if n := s.restores.Load(); n != 0 {
		t.Fatalf("a gap of noops and barriers must not restore the legacy snapshot (%d restores)", n)
	}

	state := appliedState(t, s.applier)
	if state.Index != 7 || state.Term != 2 || state.ConfigurationIndex != 1 {
		t.Fatalf("migrated state = %+v, want index 7, term 2, configuration index 1", state)
	}

	if _, err := os.Stat(filepath.Join(s.raftDir, legacySnapshotsDir)); !os.IsNotExist(err) {
		t.Fatalf("legacy snapshot directory must be removed: %v", err)
	}
}

func TestLegacyMigrationRestoresWhenDatabaseMissesCommands(t *testing.T) {
	s := newLegacyStore(t, 5)
	s.log(t, command(5, 1), command(6, 1), command(7, 1))
	s.snapshot(t, 7, 1, legacyPayload(t, 9))

	if _, err := s.migrate(t); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if n := s.restores.Load(); n != 1 {
		t.Fatalf("a database behind the legacy snapshot must restore it once, got %d restores", n)
	}

	if n := rowCount(t, s.applier); n != 9 {
		t.Fatalf("restored database has %d rows, want the snapshot's 9", n)
	}

	if state := appliedState(t, s.applier); state.Index != 7 || state.Term != 1 {
		t.Fatalf("migrated state = %+v, want index 7, term 1", state)
	}
}

func TestLegacyMigrationReadsPositionPastSnapshotFromLog(t *testing.T) {
	s := newLegacyStore(t, 9)
	s.log(t,
		command(3, 1), command(4, 1), command(5, 2), command(6, 2), command(7, 2),
		configurationEntry(8, 3, grownConfiguration), command(9, 3), command(10, 3),
	)
	s.snapshot(t, 3, 1, legacyPayload(t, 1))

	if _, err := s.migrate(t); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	state := appliedState(t, s.applier)
	if state.Index != 9 || state.Term != 3 || state.ConfigurationIndex != 8 {
		t.Fatalf("migrated state = %+v, want index 9, term 3, configuration index 8", state)
	}

	if servers := hraft.DecodeConfiguration(state.Configuration).Servers; len(servers) != 2 {
		t.Fatalf("migrated configuration has %d servers, want the configuration committed at index 8", len(servers))
	}
}

func TestLegacyMigrationWithoutSnapshotUsesLog(t *testing.T) {
	s := newLegacyStore(t, 2)
	s.log(t, configurationEntry(1, 1, legacyConfiguration), command(2, 1))

	migrated, err := s.migrate(t)
	if err != nil || !migrated {
		t.Fatalf("migrate: migrated=%v err=%v", migrated, err)
	}

	if state := appliedState(t, s.applier); state.Index != 2 || state.Term != 1 || state.ConfigurationIndex != 1 {
		t.Fatalf("migrated state = %+v, want index 2, term 1, configuration index 1", state)
	}
}

func TestLegacyMigrationRefusesDatabaseAheadOfLog(t *testing.T) {
	s := newLegacyStore(t, 9)

	_, err := s.migrate(t)
	if err == nil || !strings.Contains(err.Error(), "database is ahead of the raft store") {
		t.Fatalf("want a database-ahead error, got %v", err)
	}
}

func TestLegacyMigrationRunsOnce(t *testing.T) {
	s := newLegacyStore(t, 2)
	s.log(t, configurationEntry(1, 1, legacyConfiguration), command(2, 1))

	if migrated, err := s.migrate(t); err != nil || !migrated {
		t.Fatalf("first migrate: migrated=%v err=%v", migrated, err)
	}

	migrated, err := s.migrate(t)
	if err != nil || migrated {
		t.Fatalf("second migrate: migrated=%v err=%v", migrated, err)
	}

	if n := s.restores.Load(); n != 0 {
		t.Fatalf("an already migrated store must not restore (%d restores)", n)
	}
}

func TestLegacyMigrationRecomputesStateLeftStaleByOldBinary(t *testing.T) {
	s := newLegacyStore(t, 0)

	if err := overwriteAppliedState(t.Context(), s.applier.PlainDB(), AppliedState{
		Index:              6,
		Term:               1,
		Configuration:      hraft.EncodeConfiguration(legacyConfiguration),
		ConfigurationIndex: 1,
	}); err != nil {
		t.Fatalf("seed state from a new-format install: %v", err)
	}

	if err := s.fsm.writeLastApplied(9); err != nil {
		t.Fatalf("seed old binary lastApplied: %v", err)
	}

	s.log(t,
		command(6, 1), command(7, 2),
		configurationEntry(8, 3, grownConfiguration), command(9, 3),
	)
	s.snapshot(t, 6, 1, legacyPayload(t, 1))

	migrated, err := s.migrate(t)
	if err != nil || !migrated {
		t.Fatalf("migrate: migrated=%v err=%v", migrated, err)
	}

	state := appliedState(t, s.applier)
	if state.Index != 9 || state.Term != 3 || state.ConfigurationIndex != 8 {
		t.Fatalf("state = %+v, want index 9, term 3, configuration index 8 recomputed from the log", state)
	}

	if servers := hraft.DecodeConfiguration(state.Configuration).Servers; len(servers) != 2 {
		t.Fatalf("configuration has %d servers, want the 2 committed by the old binary", len(servers))
	}

	if _, err := os.Stat(filepath.Join(s.raftDir, legacySnapshotsDir)); !os.IsNotExist(err) {
		t.Fatalf("legacy snapshot directory must be removed: %v", err)
	}

	if n := s.restores.Load(); n != 0 {
		t.Fatalf("an up-to-date database must not restore (%d restores)", n)
	}
}

func TestLegacyMigrationRestoresWhenGapIsCompacted(t *testing.T) {
	s := newLegacyStore(t, 5)
	s.log(t, command(8, 2), command(9, 2))
	s.snapshot(t, 7, 2, legacyPayload(t, 4))

	migrated, err := s.migrate(t)
	if err != nil || !migrated {
		t.Fatalf("migrate: migrated=%v err=%v", migrated, err)
	}

	if n := s.restores.Load(); n != 1 {
		t.Fatalf("a gap that the compacted log cannot prove empty must restore the legacy snapshot, got %d restores", n)
	}

	if n := rowCount(t, s.applier); n != 4 {
		t.Fatalf("restored database has %d rows, want the snapshot's 4", n)
	}

	state := appliedState(t, s.applier)
	if state.Index != 7 || state.Term != 2 || state.ConfigurationIndex != 1 {
		t.Fatalf("migrated state = %+v, want index 7, term 2, configuration index 1", state)
	}

	if _, err := os.Stat(filepath.Join(s.raftDir, legacySnapshotsDir)); !os.IsNotExist(err) {
		t.Fatalf("legacy snapshot directory must be removed: %v", err)
	}
}

func TestLegacyMigrationRestoresWhenGapRunsPastLog(t *testing.T) {
	s := newLegacyStore(t, 5)
	s.log(t, command(5, 1), &hraft.Log{Index: 6, Term: 2, Type: hraft.LogNoop})
	s.snapshot(t, 7, 2, legacyPayload(t, 4))

	if _, err := s.migrate(t); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if n := s.restores.Load(); n != 1 {
		t.Fatalf("a gap past the end of the log must restore the legacy snapshot, got %d restores", n)
	}

	if state := appliedState(t, s.applier); state.Index != 7 || state.Term != 2 {
		t.Fatalf("migrated state = %+v, want index 7, term 2", state)
	}
}
