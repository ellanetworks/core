// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func newPinState(bootstrap map[string]int) *pkiState {
	p := &pkiState{}
	if bootstrap != nil {
		p.bootstrapPins.Store(&bootstrap)
	}

	return p
}

func TestRefreshPinsDoesNotWipeBootstrapPins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	defer func() { _ = database.Close() }()

	p := newPinState(map[string]int{"leader-fp": 1, "self-fp": 2})

	if err := p.RefreshPins(ctx, database); err != nil {
		t.Fatalf("RefreshPins: %v", err)
	}

	got := p.PinFunc()("leader-fp")
	if !got.Found {
		t.Fatal("bootstrap pin was destroyed by a refresh against a table that has not replicated yet")
	}

	if got.NodeID != 1 {
		t.Errorf("NodeID = %d, want 1", got.NodeID)
	}
}

func TestReplicatedPinsSupersedeBootstrap(t *testing.T) {
	t.Parallel()

	p := newPinState(map[string]int{"revoked-fp": 9})

	replicated := map[string]int{"live-fp": 1}
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

	if got := newPinState(nil).PinFunc()("anything"); got.Found || got.CacheSize != 0 {
		t.Errorf("want an empty result with no pin sources, got %+v", got)
	}
}
