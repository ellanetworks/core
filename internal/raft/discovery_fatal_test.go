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

func TestStartDiscoveryWithoutClusterListenerIsTerminal(t *testing.T) {
	t.Parallel()

	m := &Manager{raftID: "3"}
	m.discoveryPending.Store(true)

	err := m.StartDiscovery(context.Background())
	if err == nil {
		t.Fatal("a missing cluster listener can never resolve by retrying; it must be reported as terminal")
	}

	if !errors.Is(err, ErrDiscoveryFatal) {
		t.Errorf("error must be terminal, got %v", err)
	}

	if !strings.Contains(err.Error(), "cluster listener") {
		t.Errorf("discovery error should name the missing listener, got %q", err)
	}
}

func TestStartDiscoveryStopsOnFounderBootstrapFailure(t *testing.T) {
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

	pki := testutil.GenTestPKI(t, []string{"1"})

	mgr.attachClusterListener(listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "1",
		Pin:              pki.PinFunc(),
		Leaf:             pki.LeafFunc("1"),
	}))

	err = mgr.StartDiscovery(ctx)
	if err == nil {
		t.Fatal("a founder that cannot bootstrap must report it instead of retrying forever")
	}

	if !errors.Is(err, ErrDiscoveryFatal) {
		t.Errorf("error must be terminal, got %v", err)
	}

	if !strings.Contains(err.Error(), "bootstrap") {
		t.Errorf("discovery error should explain the bootstrap failure, got %q", err)
	}
}
