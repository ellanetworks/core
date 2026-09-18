// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegrationHADrainResumeFromFollower(t *testing.T) {
	suites.Require(t, suites.HA)

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

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

	leaderIdx, _, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	targetID, _, err := findFollower(ctx, clients)
	if err != nil {
		t.Fatalf("find follower: %v", err)
	}

	caller, err := followerOtherThan(ctx, clients, leaderIdx, targetID)
	if err != nil {
		t.Fatalf("find a follower to issue the request from: %v", err)
	}

	resp, err := caller.DrainClusterMember(ctx, targetID)
	if err != nil {
		t.Fatalf("drain node %d via a follower: %v", targetID, err)
	}

	if resp.DrainState != "draining" && resp.DrainState != "drained" {
		t.Fatalf("drainState = %q, want draining or drained", resp.DrainState)
	}

	if err := waitForDrained(ctx, caller, targetID); err != nil {
		t.Fatalf("drain issued on a follower never completed: %v", err)
	}

	if err := caller.ResumeClusterMember(ctx, targetID); err != nil {
		t.Fatalf("resume node %d via a follower: %v", targetID, err)
	}

	state, err := drainStateOf(ctx, caller, targetID)
	if err != nil {
		t.Fatalf("read drainState after resume: %v", err)
	}

	if state != "active" {
		t.Fatalf("drainState = %q after resume via a follower, want active", state)
	}

	assertMembershipConsistent(t, ctx, clients)
}

func TestIntegrationHADrainLeaderFromFollower(t *testing.T) {
	suites.Require(t, suites.HA)

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

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

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	leaderStatus, err := leader.GetStatus(ctx)
	if err != nil || leaderStatus.Cluster == nil {
		t.Fatalf("read leader status: %v", err)
	}

	leaderNodeID := leaderStatus.Cluster.NodeID

	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	if _, err := survivors[0].DrainClusterMember(ctx, leaderNodeID); err != nil {
		t.Fatalf("drain the leader via a follower: %v", err)
	}

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("leadership never moved off the drained leader: %v", err)
	}

	if err := waitForDrained(ctx, newLeader, leaderNodeID); err != nil {
		t.Fatalf("drained leader never completed its drain: %v", err)
	}

	if err := survivors[0].RemoveClusterMember(ctx, leaderNodeID, false); err != nil {
		t.Fatalf("remove the drained leader via a follower: %v", err)
	}

	if err := waitForMemberCount(ctx, newLeader, 2, 60*time.Second); err != nil {
		t.Fatalf("cluster did not shrink to 2 members: %v", err)
	}

	assertMembershipConsistent(t, ctx, survivors)
}

func followerOtherThan(ctx context.Context, clients []*client.Client, leaderIdx, excludeNodeID int) (*client.Client, error) {
	for i, c := range clients {
		if i == leaderIdx {
			continue
		}

		status, err := c.GetStatus(ctx)
		if err != nil || status.Cluster == nil {
			continue
		}

		if status.Cluster.NodeID == excludeNodeID {
			continue
		}

		return c, nil
	}

	return nil, errNoSpareFollower
}

var errNoSpareFollower = errors.New("no follower available other than the leader and the drain target")
