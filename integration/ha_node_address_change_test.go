package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

const haFQDNComposeFile = "compose-fqdn.yaml"

// rebindIPv4 is a fixed address in the cluster subnet, chosen well
// above the default IPAM allocation range so it does not collide with
// the leases held by the running peers.
const rebindIPv4 = "10.100.0.123"

// TestIntegrationHAFollowerReturnsOnNewAddress brings up a 3-node
// cluster whose peers are addressed by FQDN, then takes a follower
// down and brings it back on a different IP — the address change a
// pod replacement under any orchestrator would produce. The cluster
// must reconverge, and the follower's persisted RaftAddress must
// remain the FQDN.
func TestIntegrationHAFollowerReturnsOnNewAddress(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("address rebinding helper is IPv4-only")
	}

	ctx := context.Background()
	composeDir := haComposeDir
	composeFile := haFQDNComposeFile

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dc.Close(); err != nil {
			t.Logf("failed to close docker client: %v", err)
		}
	}()

	clients, err := bringUpHAFQDNClusterAt(t, ctx, dc, composeDir, composeFile, haNodeServices)
	if err != nil {
		t.Fatalf("bring up FQDN cluster: %v", err)
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
	expectedRaftAddr := fmt.Sprintf("%s:7000", followerService)

	container, err := dc.ResolveComposeContainer(ctx, haComposeProject, followerService)
	if err != nil {
		t.Fatalf("resolve follower container: %v", err)
	}

	networkName, oldIP, err := dc.ContainerNetworkEndpoint(ctx, container, "cluster")
	if err != nil {
		t.Fatalf("inspect follower: %v", err)
	}

	if oldIP == rebindIPv4 {
		t.Fatalf("follower already at rebind target %s; pick a different IP", rebindIPv4)
	}

	t.Logf("rebinding %s (node %d) from %s to %s on %s",
		followerService, followerNodeID, oldIP, rebindIPv4, networkName)

	if err := dc.ComposeStopWithFile(ctx, composeDir, composeFile, followerService); err != nil {
		t.Fatalf("stop follower: %v", err)
	}

	if err := dc.NetworkDisconnectContainer(ctx, networkName, container); err != nil {
		t.Fatalf("disconnect follower: %v", err)
	}

	if err := dc.NetworkConnectContainerWithIPv4(ctx, networkName, container, rebindIPv4); err != nil {
		t.Fatalf("reconnect follower at %s: %v", rebindIPv4, err)
	}

	if err := dc.ComposeStartWithFile(ctx, composeDir, composeFile, followerService); err != nil {
		t.Fatalf("start follower: %v", err)
	}

	_, newIP, err := dc.ContainerNetworkEndpoint(ctx, container, "cluster")
	if err != nil {
		t.Fatalf("re-inspect follower: %v", err)
	}

	if newIP != rebindIPv4 {
		t.Fatalf("rebind IP = %s, want %s", newIP, rebindIPv4)
	}

	if newIP == oldIP {
		t.Fatalf("rebind did not change IP (still %s)", oldIP)
	}

	if err := waitForClusterReadyWithin(ctx, clients, 90*time.Second); err != nil {
		t.Fatalf("cluster did not reconverge after follower address change: %v", err)
	}

	if _, err := waitForAutopilotHealthy(ctx, leader, 1, len(clients)); err != nil {
		t.Fatalf("autopilot did not report healthy after follower address change: %v", err)
	}

	members, err := leader.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list cluster members: %v", err)
	}

	var got string

	for _, m := range members {
		if m.NodeID == followerNodeID {
			got = m.RaftAddress
			break
		}
	}

	if got != expectedRaftAddr {
		t.Fatalf("node %d raftAddress = %q, want %q (persisted address must remain the FQDN across IP changes)",
			followerNodeID, got, expectedRaftAddr)
	}

	assertMembershipConsistent(t, ctx, clients)
}
