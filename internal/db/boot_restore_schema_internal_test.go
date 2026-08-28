// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/hashicorp/raft"
)

func buildBaselineSnapshotPayload(t *testing.T) []byte {
	t.Helper()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "snapshot-source.db")

	conn, err := openSQLiteConnection(ctx, path)
	if err != nil {
		t.Fatalf("open snapshot source: %v", err)
	}

	if err := runMigrations(ctx, conn, baselineVersion); err != nil {
		_ = conn.Close()

		t.Fatalf("migrate snapshot source to baseline: %v", err)
	}

	if err := ensureFsmStateTable(ctx, conn); err != nil {
		_ = conn.Close()

		t.Fatalf("ensure fsm_state on snapshot source: %v", err)
	}

	vacuumed := filepath.Join(t.TempDir(), "snapshot.db")
	if _, err := conn.ExecContext(ctx, "VACUUM INTO ?", vacuumed); err != nil {
		_ = conn.Close()

		t.Fatalf("vacuum snapshot source: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close snapshot source: %v", err)
	}

	payload, err := os.ReadFile(vacuumed)
	if err != nil {
		t.Fatalf("read snapshot payload: %v", err)
	}

	return payload
}

func seedRaftSnapshot(t *testing.T, dataDir string, nodeID int, payload []byte) {
	t.Helper()

	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatalf("create raft dir: %v", err)
	}

	store, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
	if err != nil {
		t.Fatalf("create snapshot store: %v", err)
	}

	_, transport := raft.NewInmemTransport("")

	configuration := raft.Configuration{
		Servers: []raft.Server{{
			Suffrage: raft.Voter,
			ID:       raft.ServerID("1"),
			Address:  raft.ServerAddress("127.0.0.1:17000"),
		}},
	}

	sink, err := store.Create(raft.SnapshotVersionMax, 10, 2, configuration, 1, transport)
	if err != nil {
		t.Fatalf("create snapshot sink: %v", err)
	}

	if _, err := sink.Write(payload); err != nil {
		_ = sink.Cancel()

		t.Fatalf("write snapshot payload: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close snapshot sink: %v", err)
	}

	nodeIDPath := filepath.Join(dataDir, "node-id")
	if err := os.WriteFile(nodeIDPath, []byte(strconv.Itoa(nodeID)+"\n"), 0o600); err != nil {
		t.Fatalf("write node-id: %v", err)
	}
}

// TestBootSnapshotRestoreRespectsBaselineInClusterMode pins the migration
// floor across a boot-time snapshot restore in HA mode: post-baseline
// migrations are proposed through Raft by the leader, so a restore that
// happens while Raft is still being constructed must not run them locally.
func TestBootSnapshotRestoreRespectsBaselineInClusterMode(t *testing.T) {
	if SchemaVersion() <= baselineVersion {
		t.Skipf("no post-baseline migration exists (schema %d, baseline %d)", SchemaVersion(), baselineVersion)
	}

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "ella.db")

	payload := buildBaselineSnapshotPayload(t)
	seedRaftSnapshot(t, dataDir, 1, payload)

	cfg := ellaraft.FastTestConfig()
	cfg.Enabled = true
	cfg.NodeID = 1
	cfg.BindAddress = "127.0.0.1:17000"
	cfg.AdvertiseAddress = "127.0.0.1:17000"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := NewDatabase(ctx, dbPath, cfg)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	applied, err := database.CurrentSchemaVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}

	if applied != baselineVersion {
		t.Fatalf("applied schema after boot-time restore = %d, want %d (baseline); "+
			"post-baseline migrations must be proposed through Raft, not applied locally",
			applied, baselineVersion)
	}
}
