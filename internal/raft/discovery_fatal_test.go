// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
)

func TestStartDiscoveryWithoutClusterListenerIsTerminal(t *testing.T) {
	t.Parallel()

	m := &Manager{nodeID: 3}
	m.discoveryPending.Store(true)

	m.StartDiscovery(context.Background())

	if got := m.DiscoveryError(); got == "" {
		t.Fatal("a missing cluster listener can never resolve by retrying; it must be reported as terminal")
	}

	if !strings.Contains(m.DiscoveryError(), "cluster listener") {
		t.Errorf("discovery error should name the missing listener, got %q", m.DiscoveryError())
	}
}

func TestRunDiscoveryStopsOnTerminalError(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, FastTestConfig(), applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	t.Cleanup(func() { _ = mgr.Shutdown() })

	if err := mgr.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	mgr.config.HasJoinToken = false
	mgr.discoveryPending.Store(true)

	pki := testutil.GenTestPKI(t, []int{1})

	mgr.attachClusterListener(listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           1,
		Pin:              pki.PinFunc(),
		Leaf:             pki.LeafFunc(1),
	}))

	mgr.StartDiscovery(ctx)

	deadline := time.Now().Add(10 * time.Second)
	for mgr.DiscoveryError() == "" {
		if time.Now().After(deadline) {
			t.Fatal("discovery kept retrying an error that can never succeed instead of stopping")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if !strings.Contains(mgr.DiscoveryError(), "bootstrap") {
		t.Errorf("discovery error should explain the bootstrap failure, got %q", mgr.DiscoveryError())
	}
}
