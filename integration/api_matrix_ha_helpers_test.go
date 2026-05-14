package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// haMatrixEnv holds the three clients of a healthy 3-node cluster plus
// a resolved leader reference. The cluster is brought up once per
// TestAPIMatrixHA invocation; each registered resource runs as a
// subtest against the same env.
type haMatrixEnv struct {
	Clients   []*client.Client
	Leader    *client.Client
	LeaderIdx int
}

// setupHAMatrixEnv stages a 3-node HA cluster, waits for readiness, and
// resolves the current leader. Compose teardown and diagnostics are
// registered on t.
func setupHAMatrixEnv(ctx context.Context, t *testing.T) *haMatrixEnv {
	t.Helper()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	clients, err := bringUpHACluster(t, ctx, dc)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dc, haComposeDir, haNodeServices, clients)
	})

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all nodes ready: %v", err)
	}

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	return &haMatrixEnv{
		Clients:   clients,
		Leader:    leader,
		LeaderIdx: leaderIdx,
	}
}

// awaitConvergence blocks until every node's AppliedIndex reaches the
// leader's current value. Call after any mutation. The follower-proxy
// path already waits for its own local apply before returning, so the
// only nodes that may lag are the two non-writers.
func awaitConvergence(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	t.Helper()

	idx, err := leaderAppliedIndex(ctx, h.Leader)
	if err != nil {
		t.Fatalf("leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, h.Clients, idx); err != nil {
		t.Fatalf("followers did not converge: %v", err)
	}
}
