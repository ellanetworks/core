package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ellanetworks/core/client"
)

// TestIntegrationHADrainResumeCycle drains a follower, resumes it,
// confirms it accepts replicated writes again, and repeats the cycle
// to catch state leaks across successive drains. Also asserts that
// resuming an already-active node is a no-op.
func TestIntegrationHADrainResumeCycle(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			t.Logf("failed to close docker client: %v", err)
		}
	}()

	clients, err := bringUpHACluster(t, ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dockerClient, haComposeDir, haNodeServices, clients)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	followerID, follower, err := findFollower(ctx, clients)
	if err != nil {
		t.Fatalf("find follower: %v", err)
	}

	if err := drainAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("first drain: %v", err)
	}

	if err := resumeAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("first resume: %v", err)
	}

	const postResumeIMSI = "001019756139950"

	if err := leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           postResumeIMSI,
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	}); err != nil {
		t.Fatalf("create subscriber after resume: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, []*client.Client{follower}, idx); err != nil {
		t.Fatalf("resumed follower did not converge: %v", err)
	}

	sub, err := follower.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: postResumeIMSI})
	if err != nil {
		t.Fatalf("read post-resume subscriber from follower: %v", err)
	}

	if sub.Imsi != postResumeIMSI {
		t.Fatalf("follower returned %q, expected %q", sub.Imsi, postResumeIMSI)
	}

	if err := drainAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("second drain: %v", err)
	}

	if err := resumeAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("second resume: %v", err)
	}

	if err := leader.ResumeClusterMember(ctx, followerID); err != nil {
		t.Fatalf("resume on active node: %v", err)
	}

	state, err := drainStateOf(ctx, leader, followerID)
	if err != nil {
		t.Fatalf("read drainState after resume: %v", err)
	}

	if state != "active" {
		t.Fatalf("drainState = %q, want active", state)
	}

	assertMembershipConsistent(t, ctx, clients)
}

func findFollower(ctx context.Context, clients []*client.Client) (int, *client.Client, error) {
	for _, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			continue
		}

		if status.Cluster == nil {
			continue
		}

		if status.Cluster.Role == "Follower" {
			return status.Cluster.NodeID, c, nil
		}
	}

	return 0, nil, fmt.Errorf("no follower found")
}

func drainAndAssert(ctx context.Context, leader *client.Client, nodeID int) error {
	resp, err := leader.DrainClusterMember(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("DrainClusterMember(%d): %w", nodeID, err)
	}

	if resp.DrainState != "drained" {
		return fmt.Errorf("drainState = %q, want drained", resp.DrainState)
	}

	return nil
}

func resumeAndAssert(ctx context.Context, leader *client.Client, nodeID int) error {
	if err := leader.ResumeClusterMember(ctx, nodeID); err != nil {
		return fmt.Errorf("ResumeClusterMember(%d): %w", nodeID, err)
	}

	state, err := drainStateOf(ctx, leader, nodeID)
	if err != nil {
		return err
	}

	if state != "active" {
		return fmt.Errorf("drainState after resume = %q, want active", state)
	}

	return nil
}

func drainStateOf(ctx context.Context, leader *client.Client, nodeID int) (string, error) {
	members, err := leader.ListClusterMembers(ctx)
	if err != nil {
		return "", fmt.Errorf("list members: %w", err)
	}

	for _, m := range members {
		if m.NodeID == nodeID {
			return m.DrainState, nil
		}
	}

	return "", fmt.Errorf("node %d not in cluster members", nodeID)
}
