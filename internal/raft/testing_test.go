// Copyright 2026 Ella Networks

package raft

import (
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

// TestNewTestManager_ElectsAndApplies proves NewTestManager reaches leader
// and round-trips a command through Raft consensus. Also acts as a smoke
// test for the helper's shutdown path.
func TestNewTestManager_ElectsAndApplies(t *testing.T) {
	a := newTestApplier(t)

	m, _ := NewTestManager(t, a)

	if !m.IsLeader() {
		t.Fatalf("expected leader, state=%v", m.State())
	}

	cmd, err := NewCommand(CmdChangeset, map[string]string{"imsi": "001"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	if _, err := m.Propose(cmd, time.Second); err != nil {
		t.Fatalf("propose: %v", err)
	}

	if got := m.AppliedIndex(); got < 1 {
		t.Fatalf("applied index did not advance: %d", got)
	}

	if len(a.seen()) != 1 {
		t.Fatalf("applier saw %d commands, want 1", len(a.seen()))
	}
}

// TestNewTestManager_ExplicitCleanupIsIdempotent verifies the returned
// cleanup func can be called before t.Cleanup fires without double-shutdown
// panics or spurious test errors.
func TestNewTestManager_ExplicitCleanupIsIdempotent(t *testing.T) {
	a := newTestApplier(t)

	m, cleanup := NewTestManager(t, a)

	if m.State() == hraft.Shutdown {
		t.Fatalf("manager shut down before test body")
	}

	cleanup()
	cleanup() // second call must not panic or error
}
