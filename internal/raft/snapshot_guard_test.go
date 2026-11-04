// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hraft "github.com/hashicorp/raft"
)

func writeGuardSnapshot(t *testing.T, store hraft.SnapshotStore, index uint64, payload []byte) {
	t.Helper()

	_, transport := hraft.NewInmemTransport("")

	sink, err := store.Create(hraft.SnapshotVersionMax, index, 1, hraft.Configuration{
		Servers: []hraft.Server{{ID: "1", Address: "127.0.0.1:1"}},
	}, 1, transport)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	if _, err := sink.Write(payload); err != nil {
		_ = sink.Cancel()

		t.Fatalf("write snapshot: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close snapshot: %v", err)
	}
}

func elsnPayload(schema uint32) []byte {
	payload := make([]byte, snapshotHeaderSize+16)
	writeSnapshotHeader(payload, schema, 1)
	copy(payload[snapshotHeaderSize:], sqliteMagic)

	return payload
}

const guardLiveSchema = 20

func guardFSM(t *testing.T, lastApplied uint64) *FSM {
	t.Helper()

	a := newTestApplier(t)

	if _, err := a.db.ExecContext(context.Background(),
		"CREATE TABLE schema_version (id INTEGER PRIMARY KEY, version INTEGER NOT NULL)"); err != nil {
		t.Fatalf("create schema_version: %v", err)
	}

	if _, err := a.db.ExecContext(context.Background(),
		"INSERT INTO schema_version (id, version) VALUES (1, ?)", guardLiveSchema); err != nil {
		t.Fatalf("seed schema_version: %v", err)
	}

	fsm := NewFSM(a, t.TempDir())
	if err := fsm.writeLastApplied(lastApplied); err != nil {
		t.Fatalf("seed lastApplied: %v", err)
	}

	return fsm
}

func newGuardStore(t *testing.T) (hraft.SnapshotStore, string) {
	t.Helper()

	dir := t.TempDir()

	store, err := hraft.NewFileSnapshotStore(dir, 3, os.Stderr)
	if err != nil {
		t.Fatalf("snapshot store: %v", err)
	}

	return newestSnapshotStore{SnapshotStore: store}, dir
}

func restoring() *hraft.Config {
	cfg := hraft.DefaultConfig()
	cfg.NoSnapshotRestoreOnStart = false

	return cfg
}

func guardLogs(t *testing.T, entries ...*hraft.Log) hraft.LogStore {
	t.Helper()

	logs := hraft.NewInmemStore()

	if err := logs.StoreLogs(entries); err != nil {
		t.Fatalf("store log entries: %v", err)
	}

	return logs
}

func nonCommandTail() []*hraft.Log {
	return []*hraft.Log{
		{Index: 11, Term: 2, Type: hraft.LogNoop},
		{Index: 12, Term: 2, Type: hraft.LogBarrier},
	}
}

func TestSnapshotGuardRestoresIdleLegacyStore(t *testing.T) {
	store, _ := newGuardStore(t)
	writeGuardSnapshot(t, store, 10, elsnPayload(18))

	if err := guardSnapshotRestoreOnStart(restoring(), guardFSM(t, 5), guardLogs(t, nonCommandTail()...), store); err != nil {
		t.Fatalf("an idle legacy store must restore its snapshot and re-migrate over Raft: %v", err)
	}
}

func TestSnapshotGuardRefusesUnappliedCommandAfterOlderSnapshot(t *testing.T) {
	store, _ := newGuardStore(t)
	writeGuardSnapshot(t, store, 10, elsnPayload(18))

	tail := append(nonCommandTail(), &hraft.Log{Index: 13, Term: 2, Type: hraft.LogCommand, Data: []byte("{}")})

	err := guardSnapshotRestoreOnStart(restoring(), guardFSM(t, 5), guardLogs(t, tail...), store)
	if err == nil || !strings.Contains(err.Error(), "schema 18") {
		t.Fatalf("an unapplied command after an older-schema snapshot must be refused, got %v", err)
	}
}

func TestSnapshotGuardIgnoresCommandsCoveredBySnapshot(t *testing.T) {
	store, _ := newGuardStore(t)
	writeGuardSnapshot(t, store, 10, elsnPayload(18))

	logs := guardLogs(t,
		&hraft.Log{Index: 9, Term: 2, Type: hraft.LogCommand, Data: []byte("{}")},
		&hraft.Log{Index: 10, Term: 2, Type: hraft.LogCommand, Data: []byte("{}")},
	)

	if err := guardSnapshotRestoreOnStart(restoring(), guardFSM(t, 5), logs, store); err != nil {
		t.Fatalf("commands covered by the snapshot are not replayed and must not block the restore: %v", err)
	}
}

func TestSnapshotGuardAllowsMatchingSchema(t *testing.T) {
	store, _ := newGuardStore(t)
	writeGuardSnapshot(t, store, 10, elsnPayload(20))

	tail := append(nonCommandTail(), &hraft.Log{Index: 13, Term: 2, Type: hraft.LogCommand, Data: []byte("{}")})

	if err := guardSnapshotRestoreOnStart(restoring(), guardFSM(t, 5), guardLogs(t, tail...), store); err != nil {
		t.Fatalf("snapshot at the live schema must restore: %v", err)
	}
}

func TestSnapshotGuardAllowsUserRestoreImage(t *testing.T) {
	store, _ := newGuardStore(t)
	writeGuardSnapshot(t, store, 10, append([]byte(nil), sqliteMagic...))

	tail := append(nonCommandTail(), &hraft.Log{Index: 13, Term: 2, Type: hraft.LogCommand, Data: []byte("{}")})

	if err := guardSnapshotRestoreOnStart(restoring(), guardFSM(t, 5), guardLogs(t, tail...), store); err != nil {
		t.Fatalf("a raw user-restore image carries no schema stamp and must restore: %v", err)
	}
}

func TestSnapshotGuardRefusesUnreadableNewestSnapshot(t *testing.T) {
	store, dir := newGuardStore(t)
	writeGuardSnapshot(t, store, 5, elsnPayload(20))
	writeGuardSnapshot(t, store, 10, elsnPayload(20))

	metas, err := store.List()
	if err != nil || len(metas) != 1 {
		t.Fatalf("store must expose only the newest snapshot, got %d (%v)", len(metas), err)
	}

	state := filepath.Join(dir, "snapshots", metas[0].ID, "state.bin")
	if err := os.WriteFile(state, []byte("corrupted"), 0o600); err != nil {
		t.Fatalf("corrupt snapshot: %v", err)
	}

	err = guardSnapshotRestoreOnStart(restoring(), guardFSM(t, 1), guardLogs(t), store)
	if err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("a corrupt newest snapshot must fail the start instead of falling back, got %v", err)
	}
}

func TestSnapshotGuardSkippedWhenRestoreIsSkipped(t *testing.T) {
	store, _ := newGuardStore(t)
	writeGuardSnapshot(t, store, 10, elsnPayload(18))

	cfg := hraft.DefaultConfig()
	cfg.NoSnapshotRestoreOnStart = true

	tail := append(nonCommandTail(), &hraft.Log{Index: 13, Term: 2, Type: hraft.LogCommand, Data: []byte("{}")})

	if err := guardSnapshotRestoreOnStart(cfg, guardFSM(t, 20), guardLogs(t, tail...), store); err != nil {
		t.Fatalf("no restore means nothing to guard: %v", err)
	}
}
