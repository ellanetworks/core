// Copyright 2026 Ella Networks

package raft

import (
	"testing"
	"time"
)

func TestSetupTestCluster_ThreeNodes(t *testing.T) {
	applier := newTestApplier(t)

	tc := SetupTestCluster(t, 3, applier)

	if len(tc.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(tc.Nodes))
	}

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("expected a leader")
	}

	followers := 0

	for _, n := range tc.Nodes {
		if !n.IsLeader() {
			followers++
		}
	}

	if followers != 2 {
		t.Fatalf("expected 2 followers, got %d", followers)
	}
}

func TestSetupTestCluster_LeaderPropose(t *testing.T) {
	applier := newTestApplier(t)

	tc := SetupTestCluster(t, 3, applier)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	cmd := &Command{Type: 1, Payload: []byte("test")}
	if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
		t.Fatalf("propose failed: %v", err)
	}
}
