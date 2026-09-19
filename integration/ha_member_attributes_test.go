// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

const rebindAPIPort = 5010

func TestIntegrationHAFollowerRepublishesAPIAddress(t *testing.T) {
	suites.Require(t, suites.HA)

	beginHATest(t)

	ctx := context.Background()
	composeDir := haComposeDir
	composeFile := ComposeFile()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dc.Close(); err != nil {
			HALogf(t, "failed to close docker client: %v", err)
		}
	}()

	clients, err := bringUpHACluster(t, ctx, dc)
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dc, composeDir, haNodeServices, clients)
	})

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	followerIdx := (leaderIdx + 1) % len(clients)
	followerNodeID := followerIdx + 1
	followerService := haNodeServices[followerIdx]

	before, err := clusterMemberByID(ctx, leader, followerNodeID)
	if err != nil {
		t.Fatalf("read follower member row: %v", err)
	}

	wantAPIAddress := fmt.Sprintf("http://%s:%d", ClusterAddress(followerNodeID), rebindAPIPort)

	if before.APIAddress == wantAPIAddress {
		t.Fatalf("follower already advertises %s; pick a different port", wantAPIAddress)
	}

	HALogf(t, "re-addressing %s (node %d) API from %s to %s",
		followerService, followerNodeID, before.APIAddress, wantAPIAddress)

	peers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
	}

	if err := writeNodeConfigPort(composeDir, followerNodeID, peers, "", "", false, rebindAPIPort); err != nil {
		t.Fatalf("rewrite follower config: %v", err)
	}

	if err := dc.ComposeStopWithFile(ctx, composeDir, composeFile, followerService); err != nil {
		t.Fatalf("stop follower: %v", err)
	}

	if err := dc.ComposeStartWithFile(ctx, composeDir, composeFile, followerService); err != nil {
		t.Fatalf("start follower: %v", err)
	}

	after, err := waitForMemberAPIAddress(ctx, leader, followerNodeID, wantAPIAddress)
	if err != nil {
		t.Fatalf("follower never republished its API address: %v", err)
	}

	if after.RaftAddress != before.RaftAddress {
		t.Fatalf("node %d raftAddress changed from %q to %q; publishing attributes must not touch it",
			followerNodeID, before.RaftAddress, after.RaftAddress)
	}

	if after.Suffrage != before.Suffrage {
		t.Fatalf("node %d suffrage changed from %q to %q; publishing attributes must not touch it",
			followerNodeID, before.Suffrage, after.Suffrage)
	}

	if after.DrainState != before.DrainState {
		t.Fatalf("node %d drainState changed from %q to %q; publishing attributes must not touch it",
			followerNodeID, before.DrainState, after.DrainState)
	}
}

func clusterMemberByID(ctx context.Context, c *client.Client, nodeID int) (*client.ClusterMember, error) {
	members, err := c.ListClusterMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cluster members: %w", err)
	}

	for _, m := range members {
		if m.NodeID == nodeID {
			return &m, nil
		}
	}

	return nil, fmt.Errorf("node %d is not a cluster member", nodeID)
}

func waitForMemberAPIAddress(ctx context.Context, c *client.Client, nodeID int, want string) (*client.ClusterMember, error) {
	timeout := 2 * time.Minute
	deadline := time.Now().Add(timeout)

	var last string

	for time.Now().Before(deadline) {
		m, err := clusterMemberByID(ctx, c, nodeID)
		if err == nil {
			if m.APIAddress == want {
				return m, nil
			}

			last = m.APIAddress
		}

		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("node %d apiAddress = %q, want %q after %v", nodeID, last, want, timeout)
}
