// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/version"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

func TestStandaloneUpgradeMigratesSharedSchemaOverRaft(t *testing.T) {
	if SchemaVersion() <= baselineVersion+1 {
		t.Skipf("needs two post-baseline migrations (schema %d, baseline %d)", SchemaVersion(), baselineVersion)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	preUpgrade := baselineVersion + 1
	dbPath := filepath.Join(t.TempDir(), "ella.db")

	old, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open pre-upgrade database: %v", err)
	}

	if err := runMigrations(ctx, old, preUpgrade); err != nil {
		t.Fatalf("migrate pre-upgrade database: %v", err)
	}

	if err := ensureFsmStateTable(ctx, old); err != nil {
		t.Fatalf("ensure fsm_state on pre-upgrade database: %v", err)
	}

	if err := old.Close(); err != nil {
		t.Fatalf("close pre-upgrade database: %v", err)
	}

	cfg := ellaraft.FastTestConfig()
	cfg.HeartbeatTimeout = time.Second
	cfg.ElectionTimeout = time.Second

	database, err := NewDatabase(ctx, dbPath, cfg)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	database.proposeMu.Lock()

	var unlockOnce sync.Once

	unlock := func() { unlockOnce.Do(database.proposeMu.Unlock) }
	defer unlock()

	applied, err := database.CurrentSchemaVersion(ctx)
	if err != nil {
		t.Fatalf("read schema after construction: %v", err)
	}

	if applied != preUpgrade {
		t.Fatalf("schema after construction = %d, want %d: post-baseline migrations must not be applied locally", applied, preUpgrade)
	}

	if err := database.WaitForFSMCatchUp(ctx); err != nil {
		t.Fatalf("wait for leader barrier: %v", err)
	}

	short, stop := context.WithTimeout(ctx, 300*time.Millisecond)
	defer stop()

	if err := database.WaitForStandaloneSchema(short); err == nil {
		t.Fatal("schema readiness must not pass while the Raft migration proposals are held")
	}

	if applied, _ := database.CurrentSchemaVersion(ctx); applied != preUpgrade {
		t.Fatalf("schema moved to %d with Raft proposals held: migrations bypassed Raft", applied)
	}

	unlock()

	if err := database.WaitUntilReady(ctx); err != nil {
		t.Fatalf("standalone database never became ready: %v", err)
	}

	if applied, _ := database.CurrentSchemaVersion(ctx); applied != SchemaVersion() {
		t.Fatalf("standalone schema after ready = %d, want %d", applied, SchemaVersion())
	}
}

func TestStandaloneUpgradeKeepsSchemaAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "ella.db")

	first, err := NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase first boot: %v", err)
	}

	if err := first.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("first boot never became ready: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first boot: %v", err)
	}

	second, err := NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase restart: %v", err)
	}

	t.Cleanup(func() { _ = second.Close() })

	if err := second.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("restart never became ready: %v", err)
	}

	if applied, _ := second.CurrentSchemaVersion(ctx); applied != SchemaVersion() {
		t.Fatalf("standalone schema after restart = %d, want %d", applied, SchemaVersion())
	}
}

func TestStandaloneRestoreOfOlderBackupMigrates(t *testing.T) {
	if SchemaVersion() <= baselineVersion+1 {
		t.Skipf("needs a post-baseline migration (schema %d, baseline %d)", SchemaVersion(), baselineVersion)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	old := newDatabaseAtVersion(t, baselineVersion+1)
	if err := old.Initialize(ctx); err != nil {
		t.Fatalf("seed older database: %v", err)
	}

	backup, err := os.CreateTemp(t.TempDir(), "older-*.backup")
	if err != nil {
		t.Fatalf("create backup file: %v", err)
	}

	defer func() { _ = backup.Close() }()

	if err := old.Backup(ctx, backup); err != nil {
		t.Fatalf("back up older database: %v", err)
	}

	database, err := NewDatabase(ctx, filepath.Join(t.TempDir(), "ella.db"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.WaitUntilReady(ctx); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	if err := database.Restore(ctx, backup); err != nil {
		t.Fatalf("restore older backup: %v", err)
	}

	if err := database.WaitForStandaloneSchema(ctx); err != nil {
		t.Fatalf("restored database never migrated forward: %v", err)
	}
}

func elsnSnapshotPayload(t *testing.T, schema uint32) []byte {
	t.Helper()

	header := make([]byte, 16)
	copy(header, "ELSN")
	binary.BigEndian.PutUint32(header[4:8], 1)
	binary.BigEndian.PutUint32(header[8:12], schema)
	binary.BigEndian.PutUint32(header[12:16], version.ProtocolVersion())

	return append(header, buildSnapshotPayloadAtVersion(t, int(schema))...)
}

func TestStandaloneUpgradeOfIdleLegacyStore(t *testing.T) {
	const (
		snapshotSchema = baselineVersion + 1
		liveSchema     = baselineVersion + 3
		liveApplied    = 5
		legacyTerm     = 2
	)

	if SchemaVersion() < liveSchema {
		t.Skipf("needs three post-baseline migrations (schema %d, baseline %d)", SchemaVersion(), baselineVersion)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "ella.db")

	seedRaftSnapshot(t, dataDir, "1", "127.0.0.1:1", elsnSnapshotPayload(t, snapshotSchema))

	live, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open live database: %v", err)
	}

	if err := runMigrations(ctx, live, liveSchema); err != nil {
		t.Fatalf("migrate live database: %v", err)
	}

	if err := ensureFsmStateTable(ctx, live); err != nil {
		t.Fatalf("ensure fsm_state: %v", err)
	}

	if _, err := live.ExecContext(ctx, "UPDATE fsm_state SET lastApplied = ? WHERE id = 1", liveApplied); err != nil {
		t.Fatalf("set live lastApplied: %v", err)
	}

	if err := live.Close(); err != nil {
		t.Fatalf("close live database: %v", err)
	}

	store, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft", "raft.db"))
	if err != nil {
		t.Fatalf("open legacy log store: %v", err)
	}

	if err := store.SetUint64([]byte("CurrentTerm"), legacyTerm); err != nil {
		t.Fatalf("write legacy term: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close legacy log store: %v", err)
	}

	database, err := NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("an idle legacy standalone store must start: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.WaitUntilReady(ctx); err != nil {
		t.Fatalf("idle legacy store never became ready: %v", err)
	}

	if applied, _ := database.CurrentSchemaVersion(ctx); applied != SchemaVersion() {
		t.Fatalf("schema after starting the idle legacy store = %d, want %d", applied, SchemaVersion())
	}
}

func freeTCPAddress(t *testing.T) string {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}

	addr := ln.Addr().String()
	_ = ln.Close()

	return addr
}

func TestClusterConversionOfLegacyStandaloneStore(t *testing.T) {
	if SchemaVersion() < baselineVersion+3 {
		t.Skipf("needs three post-baseline migrations (schema %d, baseline %d)", SchemaVersion(), baselineVersion)
	}

	const (
		snapshotIndex = 10
		tailIndex     = 11
		legacyTerm    = 2
	)

	snapshotSchema := baselineVersion + 1
	liveSchema := baselineVersion + 3

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "ella.db")

	seedRaftSnapshot(t, dataDir, "1", "127.0.0.1:1", buildSnapshotPayloadAtVersion(t, snapshotSchema))

	live, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open live database: %v", err)
	}

	if err := runMigrations(ctx, live, liveSchema); err != nil {
		t.Fatalf("migrate live database: %v", err)
	}

	if err := ensureFsmStateTable(ctx, live); err != nil {
		t.Fatalf("ensure fsm_state: %v", err)
	}

	if _, err := live.ExecContext(ctx, "UPDATE fsm_state SET lastApplied = ? WHERE id = 1", tailIndex); err != nil {
		t.Fatalf("set live lastApplied: %v", err)
	}

	if err := live.Close(); err != nil {
		t.Fatalf("close live database: %v", err)
	}

	cmd, err := ellaraft.NewCommand(ellaraft.CmdChangeset, &bytesPayload{
		Operation:      "DeleteDynamicLeasesByNode",
		RequiredSchema: 9,
		CapturedSchema: liveSchema,
	})
	if err != nil {
		t.Fatalf("build legacy changeset: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal legacy changeset: %v", err)
	}

	boltPath := filepath.Join(dataDir, "raft", "raft.db")

	store, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("open legacy log store: %v", err)
	}

	if err := store.StoreLog(&raft.Log{Index: tailIndex, Term: legacyTerm, Type: raft.LogCommand, Data: data}); err != nil {
		t.Fatalf("write legacy log tail: %v", err)
	}

	if err := store.SetUint64([]byte("CurrentTerm"), legacyTerm); err != nil {
		t.Fatalf("write legacy term: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close legacy log store: %v", err)
	}

	addr := freeTCPAddress(t)

	cfg := ellaraft.FastTestConfig()
	cfg.Enabled = true
	cfg.RaftID = "1"
	cfg.BindAddress = addr
	cfg.AdvertiseAddress = addr

	database, err := NewDatabase(ctx, dbPath, cfg)
	if err != nil {
		t.Fatalf("NewDatabase in cluster mode over the legacy store: %v", err)
	}

	closed := false

	defer func() {
		if !closed {
			_ = database.Close()
		}
	}()

	for {
		applied, err := database.CurrentSchemaVersion(ctx)
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}

		if applied == SchemaVersion() {
			break
		}

		if applied < liveSchema {
			t.Fatalf("live schema rolled back to %d (want >= %d): conversion restored the legacy snapshot", applied, liveSchema)
		}

		if ctx.Err() != nil {
			t.Fatalf("schema stuck at %d after conversion, want %d", applied, SchemaVersion())
		}

		time.Sleep(50 * time.Millisecond)
	}

	if servers := database.RaftServers(); len(servers) != 1 || servers[0].Address != addr {
		t.Fatalf("sole server must be realigned to %s, got %+v", addr, servers)
	}

	closed = true

	if err := database.Close(); err != nil {
		t.Fatalf("close converted database: %v", err)
	}

	snaps, err := raft.NewFileSnapshotStore(filepath.Join(dataDir, "raft"), 3, os.Stderr)
	if err != nil {
		t.Fatalf("open snapshot store: %v", err)
	}

	metas, err := snaps.List()
	if err != nil || len(metas) == 0 {
		t.Fatalf("list snapshots: %d, %v", len(metas), err)
	}

	if metas[0].Index <= tailIndex {
		t.Fatalf("checkpoint snapshot index %d must cover the legacy tail (%d)", metas[0].Index, tailIndex)
	}

	_, rc, err := snaps.Open(metas[0].ID)
	if err != nil {
		t.Fatalf("open checkpoint snapshot: %v", err)
	}

	header := make([]byte, 16)
	_, err = io.ReadFull(rc, header)
	_ = rc.Close()

	if err != nil || !bytes.Equal(header[:4], []byte("ELSN")) {
		t.Fatalf("checkpoint snapshot header: %q, %v", header, err)
	}

	if schema := int(binary.BigEndian.Uint32(header[8:12])); schema < liveSchema {
		t.Fatalf("checkpoint snapshot schema %d, want >= live schema %d", schema, liveSchema)
	}

	logs, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("reopen log store: %v", err)
	}

	defer func() { _ = logs.Close() }()

	first, err := logs.FirstIndex()
	if err != nil {
		t.Fatalf("read first index: %v", err)
	}

	if first != 0 && first <= tailIndex {
		t.Fatalf("legacy log entry %d survived the checkpoint (first index %d)", tailIndex, first)
	}
}
