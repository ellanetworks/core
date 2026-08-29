// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestIntegrationHAForceRemoveUnreachableMember(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

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

	composeFile := ComposeFile()

	clients, err := bringUpHACluster(t, ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		diagCtx, diagCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer diagCancel()

		dumpClusterDiagnostics(t, diagCtx, dockerClient, haComposeDir, haNodeServices, clients)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	doomedID, _, err := findFollower(ctx, clients)
	if err != nil {
		t.Fatalf("find follower: %v", err)
	}

	doomedIdx := doomedID - 1
	doomedService := haNodeServices[doomedIdx]

	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != doomedIdx {
			survivors = append(survivors, c)
		}
	}

	HALogf(t, "stopping follower %s (node %d) so it can no longer be drained", doomedService, doomedID)

	if err := dockerClient.ComposeStopWithFile(ctx, haComposeDir, composeFile, doomedService); err != nil {
		t.Fatalf("stop %s: %v", doomedService, err)
	}

	err = leader.RemoveClusterMember(ctx, doomedID, false)
	if err == nil {
		t.Fatal("unforced RemoveClusterMember succeeded on a node that was never drained; the drain precondition is not enforced")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "drain") {
		t.Fatalf("unforced removal was refused, but not for the drain precondition: %v", err)
	}

	HALogf(t, "unforced removal correctly refused: %v", err)

	members, err := leader.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list members after refused removal: %v", err)
	}

	if len(members) != 3 {
		t.Fatalf("refused removal still changed membership: got %d members, want 3", len(members))
	}

	HALogf(t, "membership intact after refusal; force-removing node %d", doomedID)

	if err := leader.RemoveClusterMember(ctx, doomedID, true); err != nil {
		t.Fatalf("force RemoveClusterMember(%d): %v", doomedID, err)
	}

	if err := waitForMemberCount(ctx, leader, 2, 60*time.Second); err != nil {
		t.Fatalf("cluster did not shrink to 2 members after force removal: %v", err)
	}

	members, err = leader.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list members after force removal: %v", err)
	}

	for _, m := range members {
		if m.NodeID == doomedID {
			t.Fatalf("force-removed node %d is still present in cluster members", doomedID)
		}
	}

	const postRemoveIMSI = "001019756170000"

	if err := leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           postRemoveIMSI,
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	}); err != nil {
		t.Fatalf("write on the 2-node cluster after force removal: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, survivors, idx); err != nil {
		t.Fatalf("surviving follower did not converge after force removal: %v", err)
	}

	for i, c := range survivors {
		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: postRemoveIMSI})
		if err != nil {
			t.Fatalf("read post-removal subscriber from survivor %d: %v", i+1, err)
		}

		if sub.Imsi != postRemoveIMSI {
			t.Fatalf("survivor %d returned IMSI %q, want %q", i+1, sub.Imsi, postRemoveIMSI)
		}
	}

	assertMembershipConsistent(t, ctx, survivors)
}
