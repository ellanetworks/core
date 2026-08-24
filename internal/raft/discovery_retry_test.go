// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
)

func newDiscoveryTestManager(t *testing.T, peers []string) *Manager {
	t.Helper()

	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           2,
		Pin:              pki.PinFunc(),
		Leaf:             pki.LeafFunc(2),
	})

	m := &Manager{
		nodeID:          2,
		clusterListener: ln,
		config: ClusterConfig{
			Peers:            peers,
			AdvertiseAddress: "127.0.0.1:9999",
			HasJoinToken:     true,
			SchemaVersion:    9,
			JoinTimeout:      50 * time.Millisecond,
		},
	}

	m.discoveryPending.Store(true)

	return m
}

func TestStartDiscoveryRetriesPastJoinTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := newDiscoveryTestManager(t, []string{"127.0.0.1:1"})

	m.StartDiscovery(ctx)

	time.Sleep(300 * time.Millisecond)

	if !m.discoveryPending.Load() {
		t.Error("discovery must still be pending, not abandoned")
	}

	if got := m.DiscoveryError(); got != "" {
		t.Errorf("an unreachable peer is transient and must never be treated as terminal, got %q", got)
	}
}

func TestStartDiscoveryStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	m := newDiscoveryTestManager(t, []string{"127.0.0.1:1"})

	m.StartDiscovery(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(200 * time.Millisecond)

	if !m.discoveryPending.Load() {
		t.Error("cancelling must not mark discovery successful")
	}
}
