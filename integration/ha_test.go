package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestIntegrationHAClusterFormation(t *testing.T) {
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

	t.Log("bringing up staged HA cluster")

	clients, err := bringUpHACluster(ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	t.Log("cluster is ready, verifying roles")

	leaderCount := 0
	followerCount := 0

	var leaderAddress string

	for i, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			t.Fatalf("failed to get status from node %d: %v", i+1, err)
		}

		if status.Cluster == nil {
			t.Fatalf("node %d has no cluster status", i+1)
		}

		if status.Cluster.LeaderAPIAddress == "" {
			t.Fatalf("node %d reports empty leader address", i+1)
		}

		if leaderAddress == "" {
			leaderAddress = status.Cluster.LeaderAPIAddress
		} else if status.Cluster.LeaderAPIAddress != leaderAddress {
			t.Fatalf("node %d reports leader address %q, expected %q",
				i+1, status.Cluster.LeaderAPIAddress, leaderAddress)
		}

		switch status.Cluster.Role {
		case "Leader":
			leaderCount++
		case "Follower":
			followerCount++
		default:
			t.Fatalf("node %d has unexpected role %q", i+1, status.Cluster.Role)
		}
	}

	if leaderCount != 1 {
		t.Fatalf("expected 1 leader, got %d", leaderCount)
	}

	if followerCount != 2 {
		t.Fatalf("expected 2 followers, got %d", followerCount)
	}

	t.Log("roles verified: 1 leader, 2 followers")

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	t.Log("cluster initialized, waiting for all nodes to become ready")

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	t.Log("all nodes ready, verifying autopilot reports cluster healthy")

	apState, err := waitForAutopilotHealthy(ctx, leader, 1, 3)
	if err != nil {
		t.Fatalf("autopilot did not report healthy: %v", err)
	}

	if apState.LeaderNodeID == 0 {
		t.Fatalf("autopilot reports unknown leader: %+v", apState)
	}

	t.Logf("autopilot healthy: leaderNodeId=%d failureTolerance=%d voters=%v",
		apState.LeaderNodeID, apState.FailureTolerance, apState.Voters)

	t.Log("creating subscriber on leader")

	err = leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139935",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber on leader: %v", err)
	}

	t.Log("subscriber created, waiting for follower convergence")

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	err = waitForFollowerConvergence(ctx, clients, idx)
	if err != nil {
		t.Fatalf("followers did not converge: %v", err)
	}

	t.Log("followers converged, reading subscriber from each follower")

	for i, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			t.Fatalf("failed to get status from node %d: %v", i+1, err)
		}

		if status.Cluster == nil || status.Cluster.Role != "Follower" {
			continue
		}

		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{
			ID: "001019756139935",
		})
		if err != nil {
			t.Fatalf("failed to read subscriber from follower node %d: %v", i+1, err)
		}

		if sub.Imsi != "001019756139935" {
			t.Fatalf("follower node %d returned subscriber with IMSI %q, expected %q",
				i+1, sub.Imsi, "001019756139935")
		}

		t.Logf("follower node %d returned subscriber correctly", i+1)
	}

	assertMembershipConsistent(t, ctx, clients)
}

func TestIntegrationHAFollowerProxy(t *testing.T) {
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

	t.Log("bringing up staged HA cluster")

	clients, err := bringUpHACluster(ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	// Find a follower to send the write to.
	var (
		follower    *client.Client
		followerIdx int
	)

	for i, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			t.Fatalf("failed to get status from node %d: %v", i+1, err)
		}

		if status.Cluster == nil || status.Cluster.Role != "Follower" {
			continue
		}

		follower = c
		followerIdx = i + 1

		break
	}

	if follower == nil {
		t.Fatal("no follower found")
	}

	t.Logf("sending create-subscriber to follower node %d (will be proxied to leader)", followerIdx)

	// Write via the follower — the proxy middleware forwards to the leader
	// and waits for the local applied index to catch up before responding.
	err = follower.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139936",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber via follower proxy: %v", err)
	}

	t.Log("subscriber created via follower proxy, reading back from the same follower")

	// Read-your-writes: the proxy waited for the local index to catch up,
	// so we should be able to read the subscriber back immediately.
	sub, err := follower.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: "001019756139936",
	})
	if err != nil {
		t.Fatalf("failed to read subscriber from follower after proxied write: %v", err)
	}

	if sub.Imsi != "001019756139936" {
		t.Fatalf("follower returned subscriber with IMSI %q, expected %q", sub.Imsi, "001019756139936")
	}

	t.Log("read-your-writes on follower confirmed, verifying on leader")

	// Confirm the write landed on the leader as well.
	sub, err = leader.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: "001019756139936",
	})
	if err != nil {
		t.Fatalf("failed to read subscriber from leader: %v", err)
	}

	if sub.Imsi != "001019756139936" {
		t.Fatalf("leader returned subscriber with IMSI %q, expected %q", sub.Imsi, "001019756139936")
	}

	assertMembershipConsistent(t, ctx, clients)

	t.Log("leader confirmed subscriber, follower proxy write test passed")
}

func TestIntegrationHALeaderFailure(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	composeFile := ComposeFile()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			t.Logf("failed to close docker client: %v", err)
		}
	}()

	t.Log("bringing up staged HA cluster")

	clients, err := bringUpHACluster(ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	// Record the leader's node ID before stopping it, so we can match it in
	// the autopilot report afterward.
	leaderStatus, err := leader.GetStatus(ctx)
	if err != nil || leaderStatus.Cluster == nil {
		t.Fatalf("failed to read leader status pre-stop: %v", err)
	}

	stoppedNodeID := leaderStatus.Cluster.NodeID

	// Build the survivor list (all nodes except the current leader).
	survivors := make([]*client.Client, 0, 2)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	leaderService := haNodeServices[leaderIdx]
	t.Logf("stopping leader %s (node %d)", leaderService, stoppedNodeID)

	err = dockerClient.ComposeStopWithFile(ctx, haComposeDir, composeFile, leaderService)
	if err != nil {
		t.Fatalf("failed to stop leader: %v", err)
	}

	t.Log("leader stopped, waiting for re-election among survivors")

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("re-election failed: %v", err)
	}

	t.Log("new leader elected, verifying autopilot reports stopped node unhealthy")

	apAfterKill, err := waitForAutopilotReportsUnhealthy(ctx, newLeader, stoppedNodeID)
	if err != nil {
		t.Fatalf("autopilot did not flag stopped node unhealthy: %v", err)
	}

	if apAfterKill.FailureTolerance != 0 {
		t.Fatalf("expected failureTolerance=0 after losing one voter, got %d (state=%+v)",
			apAfterKill.FailureTolerance, apAfterKill)
	}

	t.Log("autopilot reflects the outage; writing subscriber via new leader")

	err = newLeader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139937",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber on new leader: %v", err)
	}

	t.Log("subscriber created, reading from both surviving nodes")

	idx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	err = waitForFollowerConvergence(ctx, survivors, idx)
	if err != nil {
		t.Fatalf("surviving follower did not converge: %v", err)
	}

	for i, c := range survivors {
		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{
			ID: "001019756139937",
		})
		if err != nil {
			t.Fatalf("failed to read subscriber from survivor %d: %v", i+1, err)
		}

		if sub.Imsi != "001019756139937" {
			t.Fatalf("survivor %d returned IMSI %q, expected %q", i+1, sub.Imsi, "001019756139937")
		}

		t.Logf("survivor %d returned subscriber correctly", i+1)
	}

	t.Logf("restarting stopped node %s", leaderService)

	err = dockerClient.ComposeStartWithFile(ctx, haComposeDir, composeFile, leaderService)
	if err != nil {
		t.Fatalf("failed to restart node: %v", err)
	}

	restartedClient := clients[leaderIdx]

	t.Log("waiting for restarted node to become ready")

	err = waitForNodeReady(ctx, restartedClient)
	if err != nil {
		t.Fatalf("restarted node did not become ready: %v", err)
	}

	t.Log("restarted node ready, waiting for it to converge")

	err = waitForFollowerConvergence(ctx, []*client.Client{restartedClient}, idx)
	if err != nil {
		t.Fatalf("restarted node did not converge: %v", err)
	}

	sub, err := restartedClient.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: "001019756139937",
	})
	if err != nil {
		t.Fatalf("failed to read subscriber from restarted node: %v", err)
	}

	if sub.Imsi != "001019756139937" {
		t.Fatalf("restarted node returned IMSI %q, expected %q", sub.Imsi, "001019756139937")
	}

	t.Log("restarted node returned subscriber correctly; verifying autopilot recovered")

	apRecovered, err := waitForAutopilotHealthy(ctx, newLeader, 1, 3)
	if err != nil {
		t.Fatalf("autopilot did not recover after node restart: %v", err)
	}

	t.Logf("autopilot recovered: failureTolerance=%d voters=%v",
		apRecovered.FailureTolerance, apRecovered.Voters)

	assertMembershipConsistent(t, ctx, clients)
}

func TestIntegrationHADrainLeadership(t *testing.T) {
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

	t.Log("bringing up staged HA cluster")

	clients, err := bringUpHACluster(ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	t.Log("draining the current leader")

	leaderStatus, err := leader.GetStatus(ctx)
	if err != nil || leaderStatus.Cluster == nil {
		t.Fatalf("failed to read leader status pre-drain: %v", err)
	}

	drainResp, err := leader.DrainClusterMember(ctx, leaderStatus.Cluster.NodeID, &client.DrainOptions{DeadlineSeconds: 30})
	if err != nil {
		t.Fatalf("DrainClusterMember failed: %v", err)
	}

	if drainResp.DrainState != "draining" && drainResp.DrainState != "drained" {
		t.Fatalf("expected drainState draining or drained, got %q", drainResp.DrainState)
	}

	t.Log("drain accepted, waiting for new leader")

	// The other two nodes should elect a new leader.
	newLeader, err := waitForNewLeader(ctx, clients)
	if err != nil {
		t.Fatalf("no new leader after drain: %v", err)
	}

	// The drained node must no longer be the leader.
	if newLeader == leader {
		t.Fatal("new leader is the same client as the drained node")
	}

	t.Log("new leader confirmed, writing subscriber via new leader")

	err = newLeader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139938",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber on new leader: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	err = waitForFollowerConvergence(ctx, clients, idx)
	if err != nil {
		t.Fatalf("followers did not converge: %v", err)
	}

	sub, err := newLeader.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: "001019756139938",
	})
	if err != nil {
		t.Fatalf("failed to read subscriber from new leader: %v", err)
	}

	if sub.Imsi != "001019756139938" {
		t.Fatalf("new leader returned IMSI %q, expected %q", sub.Imsi, "001019756139938")
	}

	assertMembershipConsistent(t, ctx, clients)

	t.Log("writes continue on new leader, drain leadership test passed")
}

func TestIntegrationHAScaleUpDown(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	const scaleUpComposeDir = "compose/ha-scaleup/"

	ipFamily := DetectIPFamily()
	t.Logf("Running HA scale-up test in %s mode", ipFamily)

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

	composeFile := ComposeFile()

	dockerClient.ComposeCleanup(ctx)
	t.Cleanup(func() {
		dockerClient.ComposeDownWithFile(ctx, scaleUpComposeDir, composeFile)
	})

	t.Log("bringing up 3-node cluster via scaleup compose")

	// Reuse bringUpHACluster's staged startup logic against the
	// scaleup compose directory. We pass the scaleup-specific service
	// names and container names by first overriding the globals the
	// helper uses — cleaner would be a parameterised helper, but the
	// integration tests run serially so the override is safe.
	// scaleup compose has a 4th peer address reachable later; include it
	// in the baseline peers list so all configs match.
	clients, err := bringUpHAClusterAt(ctx, dockerClient, scaleUpComposeDir, haNodeServices, []string{ClusterAddressWithPort(4, 7000)})
	if err != nil {
		t.Fatalf("bring up 3-node cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	t.Log("3-node cluster ready, staging + starting 4th node as nonvoter")

	fullPeers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
		ClusterAddressWithPort(4, 7000),
	}
	if err := stageAndStartJoiner(ctx, dockerClient, leader, scaleUpComposeDir,
		"ella-core-4", 4, fullPeers, "nonvoter"); err != nil {
		t.Fatalf("stage + start node 4: %v", err)
	}

	node4URL := APIAddressForCluster(4)

	node4Client, err := newInsecureClient(node4URL)
	if err != nil {
		t.Fatalf("failed to create client for node 4: %v", err)
	}

	node4Client.SetToken(clients[0].GetToken())

	t.Log("waiting for node 4 to appear as nonvoter")

	err = waitForMemberSuffrage(ctx, leader, 4, "nonvoter")
	if err != nil {
		t.Fatalf("node 4 did not join as nonvoter: %v", err)
	}

	t.Log("node 4 joined as nonvoter, promoting to voter")

	err = leader.PromoteClusterMember(ctx, 4)
	if err != nil {
		t.Fatalf("failed to promote node 4: %v", err)
	}

	err = waitForMemberSuffrage(ctx, leader, 4, "voter")
	if err != nil {
		t.Fatalf("node 4 did not become voter: %v", err)
	}

	t.Log("node 4 promoted to voter, writing subscriber on leader")

	err = leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139939",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber on leader: %v", err)
	}

	t.Log("subscriber created, waiting for node 4 to converge")

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	err = waitForFollowerConvergence(ctx, []*client.Client{node4Client}, idx)
	if err != nil {
		t.Fatalf("node 4 did not converge: %v", err)
	}

	sub, err := node4Client.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: "001019756139939",
	})
	if err != nil {
		t.Fatalf("failed to read subscriber from node 4: %v", err)
	}

	if sub.Imsi != "001019756139939" {
		t.Fatalf("node 4 returned IMSI %q, expected %q", sub.Imsi, "001019756139939")
	}

	t.Log("node 4 returned subscriber correctly, scaling back down to 3 nodes")

	// --- Scale down: drain and remove node 4 from the cluster (4 → 3) ---

	if _, err := leader.DrainClusterMember(ctx, 4, &client.DrainOptions{DeadlineSeconds: 0}); err != nil {
		t.Fatalf("failed to drain node 4: %v", err)
	}

	err = leader.RemoveClusterMember(ctx, 4, false)
	if err != nil {
		t.Fatalf("failed to remove node 4 from cluster: %v", err)
	}

	t.Log("node 4 removed from Raft, verifying cluster members")

	members, err := leader.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("failed to list cluster members: %v", err)
	}

	for _, m := range members {
		if m.NodeID == 4 {
			t.Fatal("removed node 4 still present in cluster members")
		}
	}

	if len(members) != 3 {
		t.Fatalf("expected 3 cluster members after removal, got %d", len(members))
	}

	t.Log("cluster members verified (3 members), exercising removed-node fence")

	// removedNodeFence (cluster_http_mux.go) returns 410 → 502 to the
	// caller. Cert revocation is a secondary defence; either failure
	// mode is acceptable as long as the request doesn't succeed.

	if _, err := node4Client.GetStatus(ctx); err != nil {
		t.Fatalf("removed node 4's API is unreachable, cannot exercise fence: %v", err)
	}

	const fencedIMSI = "001019756139940"

	err = node4Client.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           fencedIMSI,
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err == nil {
		t.Fatal("CreateSubscriber via removed node 4 succeeded; fence regression")
	}

	t.Logf("write via removed node correctly rejected: %v", err)

	if _, err := node4Client.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     5,
		TTLSeconds: 60,
	}); err == nil {
		t.Fatal("MintClusterJoinToken via removed node 4 succeeded; fence regression")
	}

	if _, err := leader.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: fencedIMSI,
	}); err == nil {
		t.Fatal("subscriber written via removed node was applied on the leader; fence is broken")
	}

	t.Log("removed-node fence rejected both write paths and nothing leaked to the leader")

	t.Log("stopping removed node container")

	err = dockerClient.ComposeStopWithFile(ctx, scaleUpComposeDir, composeFile, "ella-core-4")
	if err != nil {
		t.Fatalf("failed to stop ella-core-4: %v", err)
	}

	t.Log("writing subscriber on 3-node cluster after scale-down")

	err = leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139942",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber after scale-down: %v", err)
	}

	idx, err = leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	err = waitForFollowerConvergence(ctx, clients, idx)
	if err != nil {
		t.Fatalf("followers did not converge after scale-down: %v", err)
	}

	sub, err = leader.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: "001019756139942",
	})
	if err != nil {
		t.Fatalf("failed to read subscriber after scale-down: %v", err)
	}

	if sub.Imsi != "001019756139942" {
		t.Fatalf("leader returned IMSI %q, expected %q", sub.Imsi, "001019756139942")
	}

	assertMembershipConsistent(t, ctx, clients)

	t.Log("3-node cluster operational after scale-down, scale up/down test passed")
}

func TestIntegrationHAQuorumRecovery(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	composeFile := ComposeFile()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			t.Logf("failed to close docker client: %v", err)
		}
	}()

	t.Log("bringing up staged HA cluster")

	clients, err := bringUpHACluster(ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	t.Log("cluster ready, writing subscriber before total shutdown")

	err = leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139940",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	err = waitForFollowerConvergence(ctx, clients, idx)
	if err != nil {
		t.Fatalf("followers did not converge: %v", err)
	}

	container1, err := dockerClient.ResolveComposeContainer(ctx, "ha", "ella-core-1")
	if err != nil {
		t.Fatalf("failed to resolve container for ella-core-1: %v", err)
	}

	container2, err := dockerClient.ResolveComposeContainer(ctx, "ha", "ella-core-2")
	if err != nil {
		t.Fatalf("failed to resolve container for ella-core-2: %v", err)
	}

	// The raft directory is derived from db.path in the node config.
	// db.path is "/data/ella.db", so dataDir = "/data" and raftDir = "/data/raft/".
	const containerRaftDir = "/data/raft"

	t.Log("stopping all 3 nodes (total quorum loss)")

	for _, svc := range haNodeServices {
		if err := dockerClient.ComposeStopWithFile(ctx, haComposeDir, composeFile, svc); err != nil {
			t.Fatalf("failed to stop %s: %v", svc, err)
		}
	}

	// Build peers.json listing only nodes 1 and 2.
	// Format expected by hashicorp/raft ReadConfigJSON:
	//   [{"id": "<serverID>", "address": "<raft bind addr>"}]
	type recoveryPeer struct {
		ID      string `json:"id"`
		Address string `json:"address"`
	}

	peers := []recoveryPeer{
		{ID: "1", Address: ClusterAddressWithPort(1, 7000)},
		{ID: "2", Address: ClusterAddressWithPort(2, 7000)},
	}

	peersJSON, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal peers.json: %v", err)
	}

	tmpDir := t.TempDir()
	peersPath := filepath.Join(tmpDir, "peers.json")

	err = os.WriteFile(peersPath, peersJSON, 0o644)
	if err != nil {
		t.Fatalf("failed to write peers.json: %v", err)
	}

	destPath := filepath.Join(containerRaftDir, "peers.json")
	t.Logf("copying peers.json to %s in containers for nodes 1 and 2", destPath)

	err = dockerClient.CopyFileToContainer(ctx, container1, peersPath, destPath)
	if err != nil {
		t.Fatalf("failed to copy peers.json to node 1: %v", err)
	}

	err = dockerClient.CopyFileToContainer(ctx, container2, peersPath, destPath)
	if err != nil {
		t.Fatalf("failed to copy peers.json to node 2: %v", err)
	}

	t.Log("starting nodes 1 and 2 (node 3 stays down)")

	err = dockerClient.ComposeStartWithFile(ctx, haComposeDir, composeFile, "ella-core-1")
	if err != nil {
		t.Fatalf("failed to start ella-core-1: %v", err)
	}

	err = dockerClient.ComposeStartWithFile(ctx, haComposeDir, composeFile, "ella-core-2")
	if err != nil {
		t.Fatalf("failed to start ella-core-2: %v", err)
	}

	// Wait for the 2-node cluster to elect a leader.
	recoveredClients := []*client.Client{clients[0], clients[1]}

	err = waitForClusterReady(ctx, recoveredClients)
	if err != nil {
		t.Fatalf("recovered cluster not ready: %v", err)
	}

	_, recoveredLeader, err := findLeader(ctx, recoveredClients)
	if err != nil {
		t.Fatalf("no leader in recovered cluster: %v", err)
	}

	t.Log("recovered cluster has a leader, verifying data survived")

	sub, err := recoveredLeader.GetSubscriber(ctx, &client.GetSubscriberOptions{
		ID: "001019756139940",
	})
	if err != nil {
		t.Fatalf("failed to read subscriber from recovered leader: %v", err)
	}

	if sub.Imsi != "001019756139940" {
		t.Fatalf("recovered leader returned IMSI %q, expected %q", sub.Imsi, "001019756139940")
	}

	assertMembershipConsistent(t, ctx, recoveredClients)

	t.Log("data survived quorum-loss recovery, test passed")
}

// TestIntegrationHADisasterRecovery simulates the worst-case DR
// scenario: take a backup on a healthy 3-node cluster, destroy every
// voter (stop containers + drop volumes), then reconstruct the cluster
// from the archive on a fresh host.
//
// This end-to-end exercises:
//   - backup archive shape (ella.db carrying CA signing keys)
//   - maybeRestoreFromBundle + ExtractForRestore on first boot
//   - the DR self-issue path: no leaf on disk + active CA in DB ⇒
//     in-process issuer signs a fresh leaf for the restored node
//   - trust bundle seeded from the on-disk DB at startup
//   - fresh joiners authenticating against the restored cluster
func TestIntegrationHADisasterRecovery(t *testing.T) {
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

	dockerClient.ComposeCleanup(ctx)

	t.Cleanup(func() {
		dockerClient.ComposeDownWithFile(ctx, haComposeDir, ComposeFile())
	})

	t.Log("bringing up staged HA cluster")

	clients, err := bringUpHACluster(ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	// Capture the admin token before teardown — it's valid after DR
	// because api_tokens rows come back with the rest of the
	// replicated state.
	adminToken := leader.GetToken()
	if adminToken == "" {
		t.Fatal("leader client has no token; initialize flow did not wire it")
	}

	const (
		imsiPreDR  = "001019756139970"
		imsiPostDR = "001019756139971"
	)

	createSub := func(c *client.Client, imsi string) error {
		return c.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            "0eefb0893e6f1c2855a3a244c6db1277",
			OPc:            "98da19bbc55e2a5b53857d10557b1d26",
			SequenceNumber: "000000000022",
			ProfileName:    "default",
		})
	}

	t.Log("creating pre-DR subscriber")

	if err := createSub(leader, imsiPreDR); err != nil {
		t.Fatalf("create pre-DR subscriber: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clients, idx); err != nil {
		t.Fatalf("followers did not converge before backup: %v", err)
	}

	backupPath := filepath.Join(t.TempDir(), "backup.tar.gz")

	t.Logf("creating backup on leader at %s", backupPath)

	if err := leader.CreateBackup(ctx, &client.CreateBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("create backup: %v", err)
	}

	if info, err := os.Stat(backupPath); err != nil {
		t.Fatalf("stat backup: %v", err)
	} else if info.Size() == 0 {
		t.Fatal("backup file is empty")
	}

	t.Log("tearing down entire cluster (all volumes dropped)")

	dockerClient.ComposeDownWithFile(ctx, haComposeDir, ComposeFile())

	// Fresh cluster config: node 1 as founder, no join-token, same
	// peers list so joiners can later use the same address set.
	peers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
	}

	if err := writeNodeConfig(haComposeDir, 1, peers, "", ""); err != nil {
		t.Fatalf("write node 1 config: %v", err)
	}

	t.Log("creating node 1 container (not started) so we can stage restore.bundle")

	if err := dockerClient.ComposeCreateWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[0]); err != nil {
		t.Fatalf("compose create node 1: %v", err)
	}

	container1, err := dockerClient.ResolveComposeContainer(ctx, "ha", haNodeServices[0])
	if err != nil {
		t.Fatalf("resolve node 1 container: %v", err)
	}

	t.Logf("copying backup into %s:/data/restore.bundle", container1)

	if err := dockerClient.CopyFileToContainer(ctx, container1, backupPath, "/data/restore.bundle"); err != nil {
		t.Fatalf("copy restore.bundle to node 1: %v", err)
	}

	t.Log("starting node 1 — runtime should extract restore.bundle and self-issue a leaf")

	if err := dockerClient.ComposeStartWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[0]); err != nil {
		t.Fatalf("start node 1: %v", err)
	}

	restoredNode1, err := newInsecureClient(APIAddressForCluster(1))
	if err != nil {
		t.Fatalf("node 1 client: %v", err)
	}

	if err := waitForNodeReady(ctx, restoredNode1); err != nil {
		t.Fatalf("restored node 1 never became ready: %v", err)
	}

	// The admin token from the backup is still valid — api_tokens rows
	// come back with the rest of the replicated state.
	restoredNode1.SetToken(adminToken)

	t.Log("verifying pre-DR subscriber survived on the restored node")

	sub, err := restoredNode1.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsiPreDR})
	if err != nil {
		t.Fatalf("read pre-DR subscriber from restored node 1: %v", err)
	}

	if sub.Imsi != imsiPreDR {
		t.Fatalf("restored node returned IMSI %q, want %q", sub.Imsi, imsiPreDR)
	}

	t.Log("verifying restored node is a functional leader (can mint join tokens)")

	status, err := restoredNode1.GetStatus(ctx)
	if err != nil {
		t.Fatalf("status on restored node: %v", err)
	}

	if status.Cluster == nil || status.Cluster.Role != "Leader" {
		t.Fatalf("restored node role = %v, want Leader", status.Cluster)
	}

	t.Log("staging fresh joiners for nodes 2 and 3")

	for _, i := range []int{2, 3} {
		if err := stageAndStartJoiner(ctx, dockerClient, restoredNode1, haComposeDir, haNodeServices[i-1], i, peers, ""); err != nil {
			t.Fatalf("stage joiner node %d: %v", i, err)
		}
	}

	clientsAfterDR, err := newHANodeClients()
	if err != nil {
		t.Fatalf("ha node clients after DR: %v", err)
	}

	for _, c := range clientsAfterDR {
		c.SetToken(adminToken)
	}

	if err := waitForClusterReady(ctx, clientsAfterDR); err != nil {
		t.Fatalf("cluster not ready after DR: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clientsAfterDR); err != nil {
		t.Fatalf("not all nodes ready after DR: %v", err)
	}

	idx, err = leaderAppliedIndex(ctx, restoredNode1)
	if err != nil {
		t.Fatalf("leader applied index after DR: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clientsAfterDR, idx); err != nil {
		t.Fatalf("followers did not converge after DR: %v", err)
	}

	t.Log("verifying pre-DR subscriber is visible on every node")

	for i, c := range clientsAfterDR {
		got, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsiPreDR})
		if err != nil {
			t.Fatalf("read pre-DR subscriber from node %d: %v", i+1, err)
		}

		if got.Imsi != imsiPreDR {
			t.Fatalf("node %d returned IMSI %q, want %q", i+1, got.Imsi, imsiPreDR)
		}
	}

	t.Log("verifying writes resume after DR")

	if err := createSub(restoredNode1, imsiPostDR); err != nil {
		t.Fatalf("post-DR write: %v", err)
	}

	idx, err = leaderAppliedIndex(ctx, restoredNode1)
	if err != nil {
		t.Fatalf("leader applied index after post-DR write: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clientsAfterDR, idx); err != nil {
		t.Fatalf("followers did not converge after post-DR write: %v", err)
	}

	for i, c := range clientsAfterDR {
		got, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsiPostDR})
		if err != nil {
			t.Fatalf("read post-DR subscriber from node %d: %v", i+1, err)
		}

		if got.Imsi != imsiPostDR {
			t.Fatalf("node %d returned IMSI %q, want %q", i+1, got.Imsi, imsiPostDR)
		}
	}

	assertMembershipConsistent(t, ctx, clientsAfterDR)

	t.Log("disaster-recovery test passed")
}

// TestIntegrationHANetworkPartition partitions the current leader from
// the other voters by dropping cluster-port (7000) traffic via
// iptables-nft / ip6tables-nft inside its container, asserts a new
// leader elects, writes on the isolated leader fail, and the
// formerly-isolated node converges after heal.
//
// The rock ships iptables-nft but no /usr/sbin/iptables symlink
// (bare-base rocks skip update-alternatives), so we use the absolute
// path of the family-specific binary.
func TestIntegrationHANetworkPartition(t *testing.T) {
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

	t.Log("bringing up staged HA cluster")

	clients, err := bringUpHACluster(ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all nodes ready: %v", err)
	}

	leaderStatus, err := leader.GetStatus(ctx)
	if err != nil || leaderStatus.Cluster == nil {
		t.Fatalf("read leader status: %v", err)
	}

	isolatedNodeID := leaderStatus.Cluster.NodeID
	leaderService := haNodeServices[leaderIdx]

	leaderContainer, err := dockerClient.ResolveComposeContainer(ctx, "ha", leaderService)
	if err != nil {
		t.Fatalf("resolve leader container: %v", err)
	}

	survivors := make([]*client.Client, 0, 2)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	t.Logf("partitioning %s (node %d) on cluster port 7000", leaderService, isolatedNodeID)

	if err := partitionClusterPort(ctx, dockerClient, leaderContainer); err != nil {
		t.Fatalf("apply partition: %v", err)
	}

	healed := false

	t.Cleanup(func() {
		if healed {
			return
		}

		if err := healClusterPort(ctx, dockerClient, leaderContainer); err != nil {
			t.Logf("cleanup heal failed: %v", err)
		}
	})

	t.Log("partition applied, waiting for survivors to elect a new leader")

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("survivors did not elect a new leader: %v", err)
	}

	newLeaderStatus, err := newLeader.GetStatus(ctx)
	if err != nil || newLeaderStatus.Cluster == nil {
		t.Fatalf("read new leader status: %v", err)
	}

	if newLeaderStatus.Cluster.NodeID == isolatedNodeID {
		t.Fatalf("new leader node-id %d matches isolated node; partition not effective",
			isolatedNodeID)
	}

	t.Logf("new leader is node %d", newLeaderStatus.Cluster.NodeID)

	const survivorIMSI = "001019756140001"

	if err := newLeader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           survivorIMSI,
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	}); err != nil {
		t.Fatalf("write on new leader: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("new leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, survivors, idx); err != nil {
		t.Fatalf("survivor follower did not converge: %v", err)
	}

	t.Log("write to majority side succeeded, attempting write on isolated former leader")

	// 503 (propose times out before lease expiry) or 502 (proxy
	// can't reach a leader after step-down) are both acceptable;
	// success would mean split-brain.
	isolatedClient := clients[leaderIdx]
	writeCtx, writeCancel := context.WithTimeout(ctx, 30*time.Second)

	err = isolatedClient.CreateSubscriber(writeCtx, &client.CreateSubscriberOptions{
		Imsi:           "001019756140002",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})

	writeCancel()

	if err == nil {
		t.Fatal("isolated former leader accepted a write while partitioned; split-brain regression")
	}

	t.Logf("write on isolated former leader correctly rejected: %v", err)

	t.Log("healing partition")

	if err := healClusterPort(ctx, dockerClient, leaderContainer); err != nil {
		t.Fatalf("heal partition: %v", err)
	}

	healed = true

	t.Log("waiting for formerly isolated node to converge with new leader's state")

	postHealIdx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("leader applied index after heal: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, []*client.Client{isolatedClient}, postHealIdx); err != nil {
		t.Fatalf("formerly isolated node did not converge: %v", err)
	}

	sub, err := isolatedClient.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: survivorIMSI})
	if err != nil {
		t.Fatalf("read survivor IMSI from formerly isolated node: %v", err)
	}

	if sub.Imsi != survivorIMSI {
		t.Fatalf("formerly isolated node IMSI mismatch: got %q want %q", sub.Imsi, survivorIMSI)
	}

	assertMembershipConsistent(t, ctx, clients)

	t.Log("formerly isolated node converged after heal; partition test passed")
}

// Both --dport and --sport on INPUT and OUTPUT: peer connections are
// bidirectional and either side may use port 7000 as src or dst.
var clusterPortPartitionRules = [][]string{
	{"INPUT", "--dport", "7000"},
	{"INPUT", "--sport", "7000"},
	{"OUTPUT", "--dport", "7000"},
	{"OUTPUT", "--sport", "7000"},
}

// firewallBinariesForFamily picks the iptables variant matching
// IP_VERSION. iptables-nft handles IPv4 only; v4 rules in IPv6 mode
// are silently no-ops.
func firewallBinariesForFamily() []string {
	switch DetectIPFamily() {
	case IPv6Only:
		return []string{"/usr/sbin/ip6tables-nft"}
	case DualStack:
		return []string{"/usr/sbin/iptables-nft", "/usr/sbin/ip6tables-nft"}
	default:
		return []string{"/usr/sbin/iptables-nft"}
	}
}

func partitionClusterPort(ctx context.Context, dc *DockerClient, container string) error {
	for _, bin := range firewallBinariesForFamily() {
		for _, rule := range clusterPortPartitionRules {
			argv := append([]string{bin, "-A", rule[0], "-p", "tcp"}, rule[1:]...)
			argv = append(argv, "-j", "DROP")

			if _, err := dc.Exec(ctx, container, argv, false, 10*time.Second, nil); err != nil {
				return fmt.Errorf("apply %v: %w", argv, err)
			}
		}
	}

	return nil
}

func healClusterPort(ctx context.Context, dc *DockerClient, container string) error {
	var firstErr error

	for _, bin := range firewallBinariesForFamily() {
		for _, rule := range clusterPortPartitionRules {
			argv := append([]string{bin, "-D", rule[0], "-p", "tcp"}, rule[1:]...)
			argv = append(argv, "-j", "DROP")

			// Best-effort: keep removing the rest so we never leave
			// a half-partitioned node.
			if _, err := dc.Exec(ctx, container, argv, false, 10*time.Second, nil); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("delete %v: %w", argv, err)
			}
		}
	}

	return firstErr
}
