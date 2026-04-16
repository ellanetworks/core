// Copyright 2026 Ella Networks

package raft

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

func TestMaybeRecoverCluster_NoPeersFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raftDir := filepath.Join(dir, "raft")

	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatal(err)
	}

	recovered, err := maybeRecoverCluster(raftDir, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if recovered {
		t.Fatal("expected no recovery when peers.json is absent")
	}
}

func TestMaybeRecoverCluster_PeersIsDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raftDir := filepath.Join(dir, "raft")
	peersDir := filepath.Join(raftDir, peersFileName)

	if err := os.MkdirAll(peersDir, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := maybeRecoverCluster(raftDir, nil, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error when peers.json is a directory")
	}
}

func TestMaybeRecoverCluster_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raftDir := filepath.Join(dir, "raft")

	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(raftDir, peersFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := maybeRecoverCluster(raftDir, nil, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMaybeRecoverCluster_ValidRecovery(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raftDir := filepath.Join(dir, "raft")

	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatal(err)
	}

	applier := newTestApplier(t)
	fsm := NewFSM(applier, dir)
	boltPath := filepath.Join(raftDir, "raft.db")

	boltStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("create bolt store: %v", err)
	}

	snaps, err := hraft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		_ = boltStore.Close()

		t.Fatal(err)
	}

	logCache, err := hraft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		_ = boltStore.Close()

		t.Fatal(err)
	}

	addr, transport := hraft.NewInmemTransport("")

	cfg := hraft.DefaultConfig()
	cfg.LocalID = "1"
	cfg.Logger = newZapRaftLogger()
	cfg.HeartbeatTimeout = 50 * time.Millisecond
	cfg.ElectionTimeout = 50 * time.Millisecond
	cfg.LeaderLeaseTimeout = 50 * time.Millisecond
	cfg.CommitTimeout = 5 * time.Millisecond

	r, err := hraft.NewRaft(cfg, fsm, logCache, boltStore, snaps, transport)
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create raft: %v", err)
	}

	bootCfg := hraft.Configuration{
		Servers: []hraft.Server{{ID: "1", Address: addr}},
	}

	if err := r.BootstrapCluster(bootCfg).Error(); err != nil {
		_ = r.Shutdown().Error()
		_ = boltStore.Close()

		t.Fatalf("bootstrap: %v", err)
	}

	// Wait for leader.
	deadline := time.After(5 * time.Second)

	for r.State() != hraft.Leader {
		select {
		case <-deadline:
			_ = r.Shutdown().Error()
			_ = boltStore.Close()

			t.Fatal("timed out waiting for leader")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Apply a command to ensure there's real state.
	cmd, err := NewCommand(CmdChangeset, map[string]string{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Apply(data, 5*time.Second).Error(); err != nil {
		_ = r.Shutdown().Error()
		_ = boltStore.Close()

		t.Fatalf("apply: %v", err)
	}

	// Shut down cleanly.
	if err := r.Shutdown().Error(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	_ = boltStore.Close()

	// Write peers.json with a new configuration (simulating a quorum recovery
	// that changes the server set).
	type peer struct {
		ID      string `json:"id"`
		Address string `json:"address"`
	}

	peersJSON, err := json.Marshal([]peer{{ID: "1", Address: string(addr)}})
	if err != nil {
		t.Fatal(err)
	}

	peersPath := filepath.Join(raftDir, peersFileName)

	if err := os.WriteFile(peersPath, peersJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	// Reopen stores for recovery.
	boltStore2, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("reopen bolt store: %v", err)
	}

	defer func() { _ = boltStore2.Close() }()

	snaps2, err := hraft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		t.Fatal(err)
	}

	logCache2, err := hraft.NewLogCache(raftLogCacheSize, boltStore2)
	if err != nil {
		t.Fatal(err)
	}

	_, transport2 := hraft.NewInmemTransport("")

	recovered, err := maybeRecoverCluster(raftDir, cfg, fsm, logCache2, boltStore2, snaps2, transport2)
	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	if !recovered {
		t.Fatal("expected recovery to succeed")
	}

	// peers.json should have been removed.
	if _, err := os.Stat(peersPath); !os.IsNotExist(err) {
		t.Fatalf("peers.json should have been removed after recovery, stat error: %v", err)
	}
}

func TestMaybeRecoverCluster_NoExistingState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raftDir := filepath.Join(dir, "raft")

	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatal(err)
	}

	boltPath := filepath.Join(raftDir, "raft.db")

	boltStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = boltStore.Close() }()

	snaps, err := hraft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		t.Fatal(err)
	}

	logCache, err := hraft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		t.Fatal(err)
	}

	_, transport := hraft.NewInmemTransport("")

	applier := newTestApplier(t)
	fsm := NewFSM(applier, dir)

	cfg := hraft.DefaultConfig()
	cfg.LocalID = "1"
	cfg.Logger = newZapRaftLogger()

	// Write valid peers.json on an empty store (no prior Raft state).
	type peer struct {
		ID      string `json:"id"`
		Address string `json:"address"`
	}

	peersJSON, err := json.Marshal([]peer{{ID: "1", Address: "127.0.0.1:7000"}})
	if err != nil {
		t.Fatal(err)
	}

	peersPath := filepath.Join(raftDir, peersFileName)

	if err := os.WriteFile(peersPath, peersJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	// RecoverCluster refuses to recover when there's no existing state.
	_, err = maybeRecoverCluster(raftDir, cfg, fsm, logCache, boltStore, snaps, transport)
	if err == nil {
		t.Fatal("expected error when recovering without existing state")
	}

	// peers.json should NOT have been removed since recovery failed.
	if _, statErr := os.Stat(peersPath); os.IsNotExist(statErr) {
		t.Fatal("peers.json should not be removed on failed recovery")
	}
}

func TestMaybeRecoverCluster_EmptyConfiguration(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raftDir := filepath.Join(dir, "raft")

	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Empty array — no servers.
	if err := os.WriteFile(
		filepath.Join(raftDir, peersFileName),
		[]byte("[]"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg := hraft.DefaultConfig()
	cfg.LocalID = "1"
	cfg.Logger = newZapRaftLogger()

	// ReadConfigJSON should succeed (valid JSON) but RecoverCluster should
	// reject the empty configuration. We need stores for that check.
	boltPath := filepath.Join(raftDir, fmt.Sprintf("raft-%d.db", time.Now().UnixNano()))

	boltStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = boltStore.Close() }()

	snaps, err := hraft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		t.Fatal(err)
	}

	logCache, err := hraft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		t.Fatal(err)
	}

	_, transport := hraft.NewInmemTransport("")

	applier := newTestApplier(t)
	fsm := NewFSM(applier, dir)

	_, err = maybeRecoverCluster(raftDir, cfg, fsm, logCache, boltStore, snaps, transport)
	if err == nil {
		t.Fatal("expected error for empty server configuration")
	}
}
