// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

func TestSoleVoterTimeoutsReloadCleanly(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	advertise := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	cfg := ClusterConfig{
		Enabled:          true,
		BindAddress:      advertise,
		AdvertiseAddress: advertise,
		Bootstrap:        true,
	}

	m, err := NewManager(ctx, cfg, newTestApplier(t), t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	t.Cleanup(func() { _ = m.Shutdown() })

	lease := raft.DefaultConfig().LeaderLeaseTimeout * defaultPerformanceMultiplier

	if got := m.soleVoterTimeouts.HeartbeatTimeout; got != lease {
		t.Fatalf("sole-voter heartbeat = %s, want the leader-lease floor %s", got, lease)
	}

	if m.clusterTimeouts.HeartbeatTimeout <= m.soleVoterTimeouts.HeartbeatTimeout {
		t.Fatalf("cluster heartbeat %s must exceed the sole-voter %s",
			m.clusterTimeouts.HeartbeatTimeout, m.soleVoterTimeouts.HeartbeatTimeout)
	}

	if err := m.raft.ReloadConfig(m.soleVoterTimeouts); err != nil {
		t.Fatalf("sole-voter timeouts must be reloadable: %v", err)
	}

	if err := m.raft.ReloadConfig(m.clusterTimeouts); err != nil {
		t.Fatalf("cluster timeouts must be reloadable: %v", err)
	}
}

func TestRelaxAndRestoreTrackServerCount(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	advertise := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	cfg := FastTestConfig()
	cfg.Enabled = true
	cfg.BindAddress = advertise
	cfg.AdvertiseAddress = advertise
	cfg.Bootstrap = true

	m, err := NewManager(ctx, cfg, newTestApplier(t), t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	t.Cleanup(func() { _ = m.Shutdown() })

	if err := m.bootstrapCluster(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if got := m.countServers(); got != 1 {
		t.Fatalf("countServers after bootstrap = %d, want 1", got)
	}

	m.relaxSoleVoterTimeouts()

	if err := m.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("sole voter never led: %v", err)
	}

	m.restoreClusterTimeouts()

	if got := m.countServers(); got != 1 {
		t.Fatalf("countServers = %d, want 1", got)
	}
}
