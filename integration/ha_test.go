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

	err = waitForClusterReady(ctx, clients, 3)
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
