// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/google/uuid"
)

// TestIntegrationHANodeIdentity covers the identity a node assigns
// itself: every member of a freshly formed cluster holds a distinct
// UUID it generated on first boot, that identity survives a restart,
// and the display name an operator sets rides alongside it.
func TestIntegrationHANodeIdentity(t *testing.T) {
	suites.Require(t, suites.HA)

	beginHATest(t)

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			HALogf(t, "failed to close docker client: %v", err)
		}
	}()

	clients, err := bringUpHACluster(t, ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dockerClient, haComposeDir, haNodeServices, clients)
	})

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	members, err := leader.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list cluster members: %v", err)
	}

	if len(members) != len(haNodeServices) {
		t.Fatalf("expected %d members, got %d", len(haNodeServices), len(members))
	}

	seenID := make(map[client.NodeID]string, len(members))

	for _, m := range members {
		if _, err := uuid.Parse(m.NodeID.String()); err != nil {
			t.Errorf("node at %s holds %q, want a self-generated UUID", m.RaftAddress, m.NodeID)
		}

		if prev, dup := seenID[m.NodeID]; dup {
			t.Errorf("nodes at %s and %s share the identity %s", prev, m.RaftAddress, m.NodeID)
		}

		seenID[m.NodeID] = m.RaftAddress

		if m.DisplayName != "" {
			t.Errorf("node %s starts with display name %q, want empty", m.NodeID, m.DisplayName)
		}
	}

	HALogf(t, "three nodes formed a cluster under self-generated identities: %v", seenID)

	// An identity is the node's own and outlives the process that
	// generated it; it is read back from <dataDir>/node-id on restart.
	restartIdx, restartID, _, err := findFollower(ctx, clients)
	if err != nil {
		t.Fatalf("find follower: %v", err)
	}

	restartService := haNodeServices[restartIdx]

	if err := dockerClient.ComposeStopWithFile(ctx, haComposeDir, ComposeFile(), restartService); err != nil {
		t.Fatalf("stop %s: %v", restartService, err)
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, haComposeDir, ComposeFile(), restartService); err != nil {
		t.Fatalf("start %s: %v", restartService, err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready after restarting %s: %v", restartService, err)
	}

	afterRestart, err := nodeIDOf(ctx, clients[restartIdx])
	if err != nil {
		t.Fatalf("resolve identity of %s after restart: %v", restartService, err)
	}

	if afterRestart != restartID {
		t.Fatalf("%s came back as %s, want the persisted %s: an identity must survive a restart",
			restartService, afterRestart, restartID)
	}

	HALogf(t, "%s kept identity %s across a restart", restartService, restartID)

	if err := leader.SetClusterMemberDisplayName(ctx, restartID, "edge-rack-4"); err != nil {
		t.Fatalf("set display name: %v", err)
	}

	named, err := leader.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list cluster members after rename: %v", err)
	}

	for _, m := range named {
		switch {
		case m.NodeID == restartID && m.DisplayName != "edge-rack-4":
			t.Fatalf("node %s display name = %q, want edge-rack-4", m.NodeID, m.DisplayName)
		case m.NodeID != restartID && m.DisplayName != "":
			t.Fatalf("renaming one node also renamed %s to %q", m.NodeID, m.DisplayName)
		}
	}

	HALog(t, "display name applied to exactly one node")
}
