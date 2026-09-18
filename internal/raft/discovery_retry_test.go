// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"strings"
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

func TestStartDiscoveryGivesUpAfterJoinTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := newDiscoveryTestManager(t, []string{"127.0.0.1:1"})

	err := m.StartDiscovery(ctx)
	if err == nil {
		t.Fatal("a joiner that never reaches a peer must give up so the supervisor restarts it")
	}

	if !errors.Is(err, ErrDiscoveryFatal) {
		t.Errorf("giving up must be terminal for the caller, got %v", err)
	}

	if !m.discoveryPending.Load() {
		t.Error("a node that never joined must not be marked as formed")
	}
}

func TestStartDiscoveryStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	m := newDiscoveryTestManager(t, []string{"127.0.0.1:1"})
	m.config.JoinTimeout = time.Minute

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := m.StartDiscovery(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown must surface as a cancellation, got %v", err)
	}

	if errors.Is(err, ErrDiscoveryFatal) {
		t.Error("shutting down is not a configuration error and must not be reported as one")
	}

	if !m.discoveryPending.Load() {
		t.Error("cancelling must not mark discovery successful")
	}
}

func TestStartDiscoveryReportsWhyFormedPeersWereSkipped(t *testing.T) {
	t.Parallel()

	m, serverAddr := newProbePeerHarness(t, statusHandler(&statusClusterBlock{
		Role:          "Leader",
		NodeID:        1,
		ClusterID:     "cluster-1",
		SchemaVersion: 12,
	}))

	m.nodeID = 2
	m.config = ClusterConfig{
		Peers:            []string{serverAddr},
		AdvertiseAddress: "127.0.0.1:9999",
		HasJoinToken:     true,
		SchemaVersion:    9,
		JoinTimeout:      50 * time.Millisecond,
	}
	m.discoveryPending.Store(true)

	err := m.StartDiscovery(context.Background())
	if err == nil {
		t.Fatal("a joiner that can never accept any peer must give up")
	}

	if strings.Contains(err.Error(), "no peer") {
		t.Errorf("the peer had formed a cluster; reporting otherwise sends the operator to the wrong place: %q", err)
	}

	if !strings.Contains(err.Error(), "schema") {
		t.Errorf("error should name the schema mismatch that blocked the join, got %q", err)
	}
}
