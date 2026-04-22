package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

	dockerClient.ComposeDown(ctx, haComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
	})

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

	dockerClient.ComposeDown(ctx, haComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
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

	t.Log("leader confirmed subscriber, follower proxy write test passed")
}

func TestIntegrationHALeaderFailure(t *testing.T) {
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

	dockerClient.ComposeDown(ctx, haComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
	})

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

	err = dockerClient.ComposeStop(ctx, haComposeDir, leaderService)
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

	err = dockerClient.ComposeStart(ctx, haComposeDir, leaderService)
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

	dockerClient.ComposeDown(ctx, haComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
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

	t.Log("writes continue on new leader, drain leadership test passed")
}

func TestIntegrationHAScaleUpDown(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	const scaleUpComposeDir = "compose/ha-scaleup/"

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

	dockerClient.ComposeDown(ctx, scaleUpComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, scaleUpComposeDir)
	})

	t.Log("bringing up 3-node cluster via scaleup compose")

	// Reuse bringUpHACluster's staged startup logic against the
	// scaleup compose directory. We pass the scaleup-specific service
	// names and container names by first overriding the globals the
	// helper uses — cleaner would be a parameterised helper, but the
	// integration tests run serially so the override is safe.
	// scaleup compose has a 4th peer address reachable later; include it
	// in the baseline peers list so all configs match.
	clients, err := bringUpHAClusterAt(ctx, dockerClient, scaleUpComposeDir, haNodeServices, []string{"10.100.0.14:7000"})
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
		"10.100.0.11:7000", "10.100.0.12:7000",
		"10.100.0.13:7000", "10.100.0.14:7000",
	}
	if err := stageAndStartJoiner(ctx, dockerClient, leader, scaleUpComposeDir,
		"ella-core-4", 4, fullPeers, "nonvoter"); err != nil {
		t.Fatalf("stage + start node 4: %v", err)
	}

	node4URL := "http://10.100.0.14:5002"

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

	t.Log("cluster members verified (3 members), stopping removed node container")

	err = dockerClient.ComposeStop(ctx, scaleUpComposeDir, "ella-core-4")
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

	t.Log("3-node cluster operational after scale-down, scale up/down test passed")
}

func TestIntegrationHAQuorumRecovery(t *testing.T) {
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

	dockerClient.ComposeDown(ctx, haComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
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
		if err := dockerClient.ComposeStop(ctx, haComposeDir, svc); err != nil {
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
		{ID: "1", Address: "10.100.0.11:7000"},
		{ID: "2", Address: "10.100.0.12:7000"},
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

	err = dockerClient.ComposeStart(ctx, haComposeDir, "ella-core-1")
	if err != nil {
		t.Fatalf("failed to start ella-core-1: %v", err)
	}

	err = dockerClient.ComposeStart(ctx, haComposeDir, "ella-core-2")
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

	t.Log("data survived quorum-loss recovery, test passed")
}

// TestIntegrationHARestoreWhileLeader verifies that a backup taken on the
// leader can be restored on the leader of a live 3-node cluster, followers
// converge on the restored replicated state via InstallSnapshot, subscribers
// committed after the backup are gone, and writes resume afterwards.
func TestIntegrationHARestoreWhileLeader(t *testing.T) {
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

	dockerClient.ComposeDown(ctx, haComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
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

	const (
		imsiPreBackup   = "001019756139960"
		imsiPostBackup  = "001019756139961"
		imsiPostRestore = "001019756139962"
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

	t.Log("creating subscriber that must survive the restore")

	if err := createSub(leader, imsiPreBackup); err != nil {
		t.Fatalf("failed to create pre-backup subscriber: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clients, idx); err != nil {
		t.Fatalf("followers did not converge before backup: %v", err)
	}

	backupPath := filepath.Join(t.TempDir(), "backup.tar.gz")
	t.Logf("creating backup on leader at %s", backupPath)

	if err := leader.CreateBackup(ctx, &client.CreateBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	t.Log("creating subscriber that must NOT exist after restore")

	if err := createSub(leader, imsiPostBackup); err != nil {
		t.Fatalf("failed to create post-backup subscriber: %v", err)
	}

	idx, err = leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clients, idx); err != nil {
		t.Fatalf("followers did not converge after post-backup write: %v", err)
	}

	// Sanity: post-backup subscriber visible on every node before restore.
	for i, c := range clients {
		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsiPostBackup})
		if err != nil {
			t.Fatalf("pre-restore: failed to read post-backup subscriber from node %d: %v", i+1, err)
		}

		if sub.Imsi != imsiPostBackup {
			t.Fatalf("pre-restore: node %d returned IMSI %q, expected %q", i+1, sub.Imsi, imsiPostBackup)
		}
	}

	t.Log("triggering restore on leader")

	if err := leader.RestoreBackup(ctx, &client.RestoreBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("restore on leader: %v", err)
	}

	// The leader briefly stops serving the status endpoint while it reopens
	// its DB connection and replaces the on-disk file. Wait for the cluster
	// to stabilize before reading leader state.
	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatalf("cluster not ready after restore: %v", err)
	}

	// Raft.Restore advances the leader's applied index past the old commit
	// index and triggers InstallSnapshot for followers. Use the post-restore
	// leader applied index as the convergence target.
	_, postRestoreLeader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("post-restore: failed to find leader: %v", err)
	}

	idx, err = leaderAppliedIndex(ctx, postRestoreLeader)
	if err != nil {
		t.Fatalf("failed to get leader applied index after restore: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clients, idx); err != nil {
		t.Fatalf("followers did not converge after restore: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all nodes ready after restore: %v", err)
	}

	t.Log("verifying replicated state rolled back to backup snapshot on every node")

	for i, c := range clients {
		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsiPreBackup})
		if err != nil {
			t.Fatalf("post-restore: pre-backup subscriber missing from node %d: %v", i+1, err)
		}

		if sub.Imsi != imsiPreBackup {
			t.Fatalf("post-restore: node %d returned IMSI %q, expected %q", i+1, sub.Imsi, imsiPreBackup)
		}

		if _, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsiPostBackup}); err == nil {
			t.Fatalf("post-restore: node %d still has post-backup subscriber; restore did not roll back state", i+1)
		}
	}

	t.Log("verifying writes resume after restore")

	if err := createSub(postRestoreLeader, imsiPostRestore); err != nil {
		t.Fatalf("post-restore write failed: %v", err)
	}

	idx, err = leaderAppliedIndex(ctx, postRestoreLeader)
	if err != nil {
		t.Fatalf("failed to get leader applied index after post-restore write: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, clients, idx); err != nil {
		t.Fatalf("followers did not converge after post-restore write: %v", err)
	}

	for i, c := range clients {
		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsiPostRestore})
		if err != nil {
			t.Fatalf("post-restore write: failed to read on node %d: %v", i+1, err)
		}

		if sub.Imsi != imsiPostRestore {
			t.Fatalf("post-restore write: node %d returned IMSI %q, expected %q", i+1, sub.Imsi, imsiPostRestore)
		}
	}

	t.Log("restore-while-leader test passed")
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

	dockerClient.ComposeDown(ctx, haComposeDir)

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
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

	dockerClient.ComposeDown(ctx, haComposeDir)

	// Fresh cluster config: node 1 as founder, no join-token, same
	// peers list so joiners can later use the same address set.
	peers := []string{"10.100.0.11:7000", "10.100.0.12:7000", "10.100.0.13:7000"}

	if err := writeNodeConfig(haComposeDir, 1, peers, "", ""); err != nil {
		t.Fatalf("write node 1 config: %v", err)
	}

	t.Log("creating node 1 container (not started) so we can stage restore.bundle")

	if err := dockerClient.ComposeCreate(ctx, haComposeDir, haNodeServices[0]); err != nil {
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

	if err := dockerClient.ComposeStart(ctx, haComposeDir, haNodeServices[0]); err != nil {
		t.Fatalf("start node 1: %v", err)
	}

	restoredNode1, err := newInsecureClient(haNodeURLs[0])
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

	t.Log("disaster-recovery test passed")
}
