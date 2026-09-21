// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

func soleServer(t *testing.T, m *Manager) raft.Server {
	t.Helper()

	future := m.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("read raft configuration: %v", err)
	}

	servers := future.Configuration().Servers
	if len(servers) != 1 {
		t.Fatalf("expected exactly one server, got %d", len(servers))
	}

	return servers[0]
}

func TestStandaloneToHARealignsStaleLoopbackAddress(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()

	standalone, err := NewManager(ctx, FastTestConfig(), newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager standalone: %v", err)
	}

	if err := standalone.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("standalone barrier: %v", err)
	}

	bootstrapped := soleServer(t, standalone)
	if !strings.HasPrefix(string(bootstrapped.Address), "127.0.0.1:") {
		t.Fatalf("standalone should bootstrap on loopback, got %q", bootstrapped.Address)
	}

	if err := standalone.Shutdown(); err != nil {
		t.Fatalf("shutdown standalone: %v", err)
	}

	advertise := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	haCfg := FastTestConfig()
	haCfg.Enabled = true
	haCfg.BindAddress = advertise
	haCfg.AdvertiseAddress = advertise

	ha, err := NewManager(ctx, haCfg, newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager HA: %v", err)
	}

	t.Cleanup(func() { _ = ha.Shutdown() })

	if got := soleServer(t, ha).Address; string(got) != advertise {
		t.Fatalf("flipping to HA must rewrite the stale bootstrap address: got %q, want %q", got, advertise)
	}

	if err := ha.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("realigned node must still be able to lead: %v", err)
	}
}

func TestHARestartLeavesMatchingAddressAlone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	advertise := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	cfg := FastTestConfig()
	cfg.Enabled = true
	cfg.RaftID = "1"
	cfg.BindAddress = advertise
	cfg.AdvertiseAddress = advertise
	cfg.Bootstrap = true

	first, err := NewManager(ctx, cfg, newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := first.StartDiscovery(ctx); err == nil {
		t.Fatal("expected discovery to refuse without a cluster listener")
	}

	if err := first.raft.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{{ID: raft.ServerID("1"), Address: raft.ServerAddress(advertise)}},
	}).Error(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if err := first.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	if err := first.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	second, err := NewManager(ctx, cfg, newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager restart: %v", err)
	}

	t.Cleanup(func() { _ = second.Shutdown() })

	if got := soleServer(t, second).Address; string(got) != advertise {
		t.Fatalf("an unchanged address must survive a restart untouched: got %q, want %q", got, advertise)
	}
}
