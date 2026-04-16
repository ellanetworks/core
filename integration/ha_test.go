package integration_test

import (
	"context"
	"os"
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

	err = dockerClient.ComposeUp(ctx, haComposeDir)
	if err != nil {
		t.Fatalf("failed to bring up HA compose: %v", err)
	}

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
	})

	clients, err := newHANodeClients()
	if err != nil {
		t.Fatalf("failed to create HA node clients: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	t.Log("waiting for cluster to become ready")

	err = waitForClusterReady(ctx, clients)
	if err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	t.Log("cluster is ready, verifying roles")

	leaderCount := 0
	followerCount := 0

	for i, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			t.Fatalf("failed to get status from node %d: %v", i+1, err)
		}

		if status.Cluster == nil {
			t.Fatalf("node %d has no cluster status", i+1)
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

	err = initializeCluster(ctx, leader, clients)
	if err != nil {
		t.Fatalf("failed to initialize cluster: %v", err)
	}

	t.Log("cluster initialized, waiting for all nodes to become ready")

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	t.Log("all nodes ready, creating subscriber on leader")

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

	err = dockerClient.ComposeUp(ctx, haComposeDir)
	if err != nil {
		t.Fatalf("failed to bring up HA compose: %v", err)
	}

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
	})

	clients, err := newHANodeClients()
	if err != nil {
		t.Fatalf("failed to create HA node clients: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	t.Log("waiting for cluster to become ready")

	err = waitForClusterReady(ctx, clients)
	if err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	err = initializeCluster(ctx, leader, clients)
	if err != nil {
		t.Fatalf("failed to initialize cluster: %v", err)
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

	err = dockerClient.ComposeUp(ctx, haComposeDir)
	if err != nil {
		t.Fatalf("failed to bring up HA compose: %v", err)
	}

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
	})

	clients, err := newHANodeClients()
	if err != nil {
		t.Fatalf("failed to create HA node clients: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	t.Log("waiting for cluster to become ready")

	err = waitForClusterReady(ctx, clients)
	if err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	err = initializeCluster(ctx, leader, clients)
	if err != nil {
		t.Fatalf("failed to initialize cluster: %v", err)
	}

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	// Build the survivor list (all nodes except the current leader).
	survivors := make([]*client.Client, 0, 2)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	leaderService := haNodeServices[leaderIdx]
	t.Logf("stopping leader %s (node %d)", leaderService, leaderIdx+1)

	err = dockerClient.ComposeStop(ctx, haComposeDir, leaderService)
	if err != nil {
		t.Fatalf("failed to stop leader: %v", err)
	}

	t.Log("leader stopped, waiting for re-election among survivors")

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("re-election failed: %v", err)
	}

	t.Log("new leader elected, writing subscriber via new leader")

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

	t.Log("restarted node returned subscriber correctly, leader failure test passed")
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

	err = dockerClient.ComposeUp(ctx, haComposeDir)
	if err != nil {
		t.Fatalf("failed to bring up HA compose: %v", err)
	}

	t.Cleanup(func() {
		dockerClient.ComposeDown(ctx, haComposeDir)
	})

	clients, err := newHANodeClients()
	if err != nil {
		t.Fatalf("failed to create HA node clients: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(ctx, dockerClient, clients, t.Logf)
	})

	t.Log("waiting for cluster to become ready")

	err = waitForClusterReady(ctx, clients)
	if err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	err = initializeCluster(ctx, leader, clients)
	if err != nil {
		t.Fatalf("failed to initialize cluster: %v", err)
	}

	err = waitForAllNodesReady(ctx, clients)
	if err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	t.Log("draining the current leader")

	drainResp, err := leader.DrainNode(ctx, &client.DrainOptions{TimeoutSeconds: 30})
	if err != nil {
		t.Fatalf("DrainNode failed: %v", err)
	}

	if !drainResp.TransferredLeadership {
		t.Fatalf("expected transferredLeadership=true, got false")
	}

	t.Log("leadership transferred, waiting for new leader")

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
