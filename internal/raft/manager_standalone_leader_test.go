// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"os"
	"path/filepath"
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

func TestNewManagerStandaloneStartsWithMultiServerState(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)
	dataDir := t.TempDir()

	seedMultiServerRaftState(t, dataDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, FastTestConfig(), applier, dataDir)
	if err != nil {
		t.Fatalf("NewManager must start even when it can never elect itself: %v", err)
	}

	t.Cleanup(func() { _ = mgr.Shutdown() })

	if mgr.HasLeader() {
		t.Fatal("node with a stale three-server configuration must not report a leader")
	}
}

func TestNewManagerStandaloneDoesNotBlockOnElection(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := ClusterConfig{
		ElectionTimeout:    10 * time.Second,
		HeartbeatTimeout:   10 * time.Second,
		LeaderLeaseTimeout: 5 * time.Second,
	}

	start := time.Now()

	mgr, err := NewManager(ctx, cfg, applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	t.Cleanup(func() { _ = mgr.Shutdown() })

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("NewManager blocked for %s; startup must not wait for leadership", elapsed)
	}
}

func TestApplyTimeoutsUsesLibraryDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cfg          ClusterConfig
		singleServer bool
		heartbeat    time.Duration
		election     time.Duration
		lease        time.Duration
	}{
		{
			name:         "standalone",
			singleServer: true,
			heartbeat:    1000 * time.Millisecond,
			election:     1000 * time.Millisecond,
			lease:        500 * time.Millisecond,
		},
		{
			name:      "ha",
			cfg:       ClusterConfig{Enabled: true},
			heartbeat: 5000 * time.Millisecond,
			election:  5000 * time.Millisecond,
			lease:     2500 * time.Millisecond,
		},
		{
			name:      "ha with multiplier",
			cfg:       ClusterConfig{Enabled: true, PerformanceMultiplier: 2},
			heartbeat: 2000 * time.Millisecond,
			election:  2000 * time.Millisecond,
			lease:     1000 * time.Millisecond,
		},
		{
			name:         "explicit overrides win",
			singleServer: true,
			cfg: ClusterConfig{
				HeartbeatTimeout:   7 * time.Millisecond,
				ElectionTimeout:    8 * time.Millisecond,
				LeaderLeaseTimeout: 9 * time.Millisecond,
			},
			heartbeat: 7 * time.Millisecond,
			election:  8 * time.Millisecond,
			lease:     9 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := raft.DefaultConfig()
			applyTimeouts(rc, tt.cfg, tt.singleServer)

			if rc.HeartbeatTimeout != tt.heartbeat {
				t.Errorf("HeartbeatTimeout = %s, want %s", rc.HeartbeatTimeout, tt.heartbeat)
			}

			if rc.ElectionTimeout != tt.election {
				t.Errorf("ElectionTimeout = %s, want %s", rc.ElectionTimeout, tt.election)
			}

			if rc.LeaderLeaseTimeout != tt.lease {
				t.Errorf("LeaderLeaseTimeout = %s, want %s", rc.LeaderLeaseTimeout, tt.lease)
			}
		})
	}
}
