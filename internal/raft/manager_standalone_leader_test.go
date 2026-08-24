// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

func seedMultiServerRaftState(t testing.TB, dataDir string) {
	t.Helper()

	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatalf("create raft directory: %v", err)
	}

	boltStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft.db"))
	if err != nil {
		t.Fatalf("create bolt store: %v", err)
	}

	snaps, err := raft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create snapshot store: %v", err)
	}

	cfg := raft.DefaultConfig()
	cfg.LocalID = "1"
	cfg.Logger = newZapRaftLogger()

	_, transport := raft.NewInmemTransport("")

	configuration := raft.Configuration{
		Servers: []raft.Server{
			{ID: "1", Address: "127.0.0.1:3"},
			{ID: "2", Address: "127.0.0.1:1"},
			{ID: "3", Address: "127.0.0.1:2"},
		},
	}

	if err := raft.BootstrapCluster(cfg, boltStore, boltStore, snaps, transport, configuration); err != nil {
		_ = boltStore.Close()

		t.Fatalf("bootstrap multi-server state: %v", err)
	}

	if err := boltStore.Close(); err != nil {
		t.Fatalf("close bolt store: %v", err)
	}

	if err := writeNodeIDFile(filepath.Join(dataDir, nodeIDFilename), 1); err != nil {
		t.Fatalf("write node-id: %v", err)
	}
}

func TestNewManagerStandaloneFailsOnMultiServerState(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)
	dataDir := t.TempDir()

	seedMultiServerRaftState(t, dataDir)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, ClusterConfig{}, applier, dataDir)
	if err == nil {
		_ = mgr.Shutdown()

		t.Fatal("expected NewManager to fail when standalone state lists multiple servers")
	}

	for _, want := range []string{"did not become raft leader", "lists servers", "peers.json"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q; got: %v", want, err)
		}
	}

	for _, id := range []string{"1", "2", "3"} {
		if !strings.Contains(err.Error(), id) {
			t.Fatalf("error does not name server %q; got: %v", id, err)
		}
	}
}

func TestStandaloneLeaderTimeoutIsBounded(t *testing.T) {
	t.Parallel()

	if standaloneLeaderTimeout <= 0 || standaloneLeaderTimeout > 5*time.Minute {
		t.Fatalf("standaloneLeaderTimeout must be a sane positive bound on startup, got %s", standaloneLeaderTimeout)
	}
}
