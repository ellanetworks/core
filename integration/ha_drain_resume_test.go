package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// TestIntegrationHADrainResumeCycle exercises the drain → resume round-trip
// that operators run for routine maintenance (kernel patch, hardware swap,
// config push). The drain happy path is covered by
// TestIntegrationHADrainLeadership; the resume side has no integration
// coverage today, even though the SDK and server handler both exist.
//
// What this test pins:
//
//  1. Drain transitions a follower's drainState through draining → drained.
//  2. Resume clears drainState back to "active".
//  3. After resume, the node serves a fresh write that lands on the leader
//     (proves replication still flows to the resumed node).
//  4. Repeating the drain/resume cycle on the same node works — catches
//     state leaks (timers, watchdogs, BGP service holding a stale flag).
//  5. Resume on a non-drained node is idempotent (no error, no state churn).
//
// The test deliberately drains a follower, not the leader. Draining the
// leader is already covered by TestIntegrationHADrainLeadership; here the
// goal is the resume contract, which is the same regardless of whether
// the target is a follower or a former leader.
func TestIntegrationHADrainResumeCycle(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

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

	t.Logf("draining follower node %d", followerID)

	// First cycle.
	if err := drainAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("first drain: %v", err)
	}

	if err := resumeAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("first resume: %v", err)
	}

	// After resume, a fresh write on the leader must still replicate to
	// the resumed node. This is the contract operators rely on: "I
	// drained, did maintenance, resumed, and the node is back in
	// service." If drain accidentally left the node fenced or paused
	// replication, this read fails.
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

	// Second cycle on the same node — catches state-leak bugs where the
	// first drain leaves residue (timer not stopped, BGP flag stuck)
	// that breaks a subsequent drain.
	t.Log("running second drain/resume cycle on the same node")

	if err := drainAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("second drain: %v", err)
	}

	if err := resumeAndAssert(ctx, leader, followerID); err != nil {
		t.Fatalf("second resume: %v", err)
	}

	// Resume on an already-active node — handler short-circuits at
	// api_drain.go:213 with 200 OK. Pin that contract: it must not error
	// and must not flip drainState.
	t.Log("resuming an already-active node (idempotency check)")

	if err := leader.ResumeClusterMember(ctx, followerID); err != nil {
		t.Fatalf("idempotent resume: %v", err)
	}

	state, err := drainStateOf(ctx, leader, followerID)
	if err != nil {
		t.Fatalf("read drainState after idempotent resume: %v", err)
	}

	if state != "active" {
		t.Fatalf("drainState after idempotent resume = %q, want active", state)
	}

	assertMembershipConsistent(t, ctx, clients)
}

// findFollower returns the nodeID and client of any follower in the cluster.
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

// drainAndAssert drains nodeID via the leader and waits for the member
// list (as seen from the leader) to report drainState == "drained".
func drainAndAssert(ctx context.Context, leader *client.Client, nodeID int) error {
	resp, err := leader.DrainClusterMember(ctx, nodeID, &client.DrainOptions{DeadlineSeconds: 30})
	if err != nil {
		return fmt.Errorf("DrainClusterMember(%d): %w", nodeID, err)
	}

	if resp.DrainState != "draining" && resp.DrainState != "drained" {
		return fmt.Errorf("immediate drainState = %q, want draining or drained", resp.DrainState)
	}

	return waitForDrainState(ctx, leader, nodeID, "drained", 60*time.Second)
}

// resumeAndAssert resumes nodeID via the leader and waits for drainState
// to return to "active".
func resumeAndAssert(ctx context.Context, leader *client.Client, nodeID int) error {
	if err := leader.ResumeClusterMember(ctx, nodeID); err != nil {
		return fmt.Errorf("ResumeClusterMember(%d): %w", nodeID, err)
	}

	return waitForDrainState(ctx, leader, nodeID, "active", 30*time.Second)
}

// waitForDrainState polls ListClusterMembers on leader until the given
// nodeID reports the desired drainState, or timeout elapses.
func waitForDrainState(ctx context.Context, leader *client.Client, nodeID int, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	var last string

	for time.Now().Before(deadline) {
		state, err := drainStateOf(ctx, leader, nodeID)
		if err == nil {
			last = state
			if state == want {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return fmt.Errorf("node %d drainState = %q after %s, want %q", nodeID, last, timeout, want)
}

// drainStateOf returns the current drainState for nodeID, sourced from
// the leader's ListClusterMembers view (the authoritative one).
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
