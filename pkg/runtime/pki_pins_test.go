// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/db"
)

func newPinState(t *testing.T, bootstrap map[string]string) (*pkiState, string) {
	t.Helper()

	agent := pkiagent.NewAgent("01a0c4a0-b76e-7050-ab95-218317ecef57", "cluster-a", t.TempDir(), "")
	if err := agent.GenerateAndPersist(); err != nil {
		t.Fatalf("GenerateAndPersist: %v", err)
	}

	p := &pkiState{agent: agent}
	if bootstrap != nil {
		p.bootstrapPins.Store(&bootstrap)
	}

	return p, p.leafFingerprint()
}

func TestRefreshPinsDoesNotWipeBootstrapPins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	defer func() { _ = database.Close() }()

	p, _ := newPinState(t, map[string]string{"leader-fp": "1", "self-fp": "2"})

	if err := p.RefreshPins(ctx, database); err != nil {
		t.Fatalf("RefreshPins: %v", err)
	}

	got := p.PinFunc()("leader-fp")
	if !got.Found {
		t.Fatal("bootstrap pin was destroyed by a refresh against a table that has not replicated yet")
	}

	if got.NodeID != "1" {
		t.Errorf("NodeID = %s, want 1", got.NodeID)
	}
}

func TestReplicatedPinsSupersedeBootstrap(t *testing.T) {
	t.Parallel()

	p, self := newPinState(t, map[string]string{"revoked-fp": "9"})

	replicated := map[string]string{"live-fp": "1", self: "2"}
	p.pins.Store(&replicated)

	if p.PinFunc()("revoked-fp").Found {
		t.Error("bootstrap pin must not outlive the replicated set that omits it")
	}

	if !p.PinFunc()("live-fp").Found {
		t.Error("replicated pin should resolve")
	}
}

func TestActivePinsWithNoSourcesIsEmpty(t *testing.T) {
	t.Parallel()

	p, _ := newPinState(t, nil)

	if got := p.PinFunc()("anything"); got.Found || got.CacheSize != 0 {
		t.Errorf("want an empty result with no pin sources, got %+v", got)
	}
}

func TestBootstrapPinsTrustedUntilReplicatedStateHoldsSelf(t *testing.T) {
	t.Parallel()

	p, self := newPinState(t, map[string]string{"leader-fp": "1"})

	staleSnapshot := map[string]string{"old-leader-fp": "1"}
	p.pins.Store(&staleSnapshot)

	if !p.PinFunc()("leader-fp").Found {
		t.Fatal("a snapshot older than this node's registration must not revoke a peer it was told to trust at join")
	}

	if !p.PinFunc()("old-leader-fp").Found {
		t.Error("replicated pin should resolve")
	}

	caughtUp := map[string]string{"new-peer-fp": "3", self: "2"}
	p.pins.Store(&caughtUp)

	if p.PinFunc()("leader-fp").Found {
		t.Error("once replicated state includes this node, a pin it omits must be revoked")
	}
}
