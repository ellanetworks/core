// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

func writeStandaloneNodeConfig(composeDir string, nodeID int) error {
	cfgDir, err := filepath.Abs(filepath.Join(composeDir, "cfg", fmt.Sprintf("node%d", nodeID)))
	if err != nil {
		return fmt.Errorf("abs path %s: %w", composeDir, err)
	}

	if err := os.MkdirAll(cfgDir, 0o777); err != nil {
		return fmt.Errorf("mkdir %s: %w", cfgDir, err)
	}

	if err := os.Chmod(cfgDir, 0o777); err != nil {
		return fmt.Errorf("chmod %s: %w", cfgDir, err)
	}

	addr := ClusterAddress(nodeID)

	body := fmt.Sprintf(`logging:
  system:
    level: "debug"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "/data/ella.db"
interfaces:
  n2:
    address: %q
    port: 38412
  n3:
    name: "eth0"
  n6:
    name: "n6"
  api:
    address: %q
    port: 5002
datapath:
  attach-mode: "xdp-generic"
`, addr, addr)

	return os.WriteFile(filepath.Join(cfgDir, "core.yaml"), []byte(body), 0o644)
}

func TestIntegrationHAStandaloneGrowsIntoCluster(t *testing.T) {
	suites.Require(t, suites.HA)

	beginHATest(t)

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			HALogf(t, "failed to close docker client: %v", err)
		}
	}()

	dockerClient.ComposeCleanup(ctx)

	composeFile := ComposeFile()

	if err := writeStandaloneNodeConfig(haComposeDir, 1); err != nil {
		t.Fatalf("write standalone config: %v", err)
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, haComposeDir, composeFile, haNodeServices[0]); err != nil {
		t.Fatalf("start node 1 standalone: %v", err)
	}

	node1, err := newInsecureClient(getHANodeURLs()[0])
	if err != nil {
		t.Fatalf("client for node 1: %v", err)
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		t.Fatalf("standalone node 1 never became ready: %v", err)
	}

	standaloneStatus, err := node1.GetStatus(ctx)
	if err != nil {
		t.Fatalf("status of standalone node: %v", err)
	}

	if standaloneStatus.Cluster.Enabled {
		t.Fatal("node 1 must start standalone for this test to mean anything")
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		t.Fatalf("initialize standalone node: %v", err)
	}

	node1.SetToken(adminToken)

	if err := dockerClient.ComposeStopWithFile(ctx, haComposeDir, composeFile, haNodeServices[0]); err != nil {
		t.Fatalf("stop node 1: %v", err)
	}

	peers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
	}

	if err := writeNodeConfig(haComposeDir, 1, nil, "", ""); err != nil {
		t.Fatalf("write cluster config for node 1: %v", err)
	}

	if err := dockerClient.ComposeStartWithFile(ctx, haComposeDir, composeFile, haNodeServices[0]); err != nil {
		t.Fatalf("restart node 1 as founder: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dockerClient, haComposeDir, haNodeServices, []*client.Client{node1})
	})

	if err := waitForNodeReady(ctx, node1); err != nil {
		t.Fatalf("converted node 1 never became ready: %v", err)
	}

	convertedStatus, err := node1.GetStatus(ctx)
	if err != nil {
		t.Fatalf("status of converted node: %v", err)
	}

	if !convertedStatus.Cluster.Enabled {
		t.Fatal("node 1 should be clustered after the conversion")
	}

	if !convertedStatus.Initialized {
		t.Fatal("the conversion must preserve the data the standalone node held")
	}

	for i := 1; i < len(haNodeServices); i++ {
		nodeID := i + 1

		if err := stageAndStartJoiner(ctx, dockerClient, node1, haComposeDir, haNodeServices[i], nodeID, peers, ""); err != nil {
			t.Fatalf("join node %d to the converted cluster: %v", nodeID, err)
		}
	}

	clients, err := newHANodeClients()
	if err != nil {
		t.Fatalf("clients: %v", err)
	}

	for _, c := range clients {
		c.SetToken(adminToken)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatalf("grown cluster not ready: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	members, err := clients[0].ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}

	if len(members) != len(haNodeServices) {
		t.Fatalf("expected %d members after growth, got %d", len(haNodeServices), len(members))
	}

	for _, m := range members {
		if m.Suffrage != "voter" {
			t.Errorf("node %s joined as %q, want voter", m.NodeID, m.Suffrage)
		}
	}

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	leaderStatus, err := leader.GetStatus(ctx)
	if err != nil {
		t.Fatalf("leader status: %v", err)
	}

	if _, err := leader.DrainClusterMember(ctx, leaderStatus.Cluster.NodeID); err != nil {
		t.Fatalf("drain the leader to force a leadership change: %v", err)
	}

	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("no new leader after draining node %s: %v", leaderStatus.Cluster.NodeID, err)
	}

	newLeaderStatus, err := newLeader.GetStatus(ctx)
	if err != nil {
		t.Fatalf("new leader status: %v", err)
	}

	if newLeaderStatus.Cluster.NodeID == leaderStatus.Cluster.NodeID {
		t.Fatalf("leadership never moved off node %s", leaderStatus.Cluster.NodeID)
	}
}
