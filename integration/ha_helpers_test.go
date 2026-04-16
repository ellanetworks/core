package integration_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
)

const haComposeDir = "compose/ha/"

var haNodeServices = []string{"ella-core-1", "ella-core-2", "ella-core-3"}

var haNodeURLs = []string{
	"http://10.100.0.11:5002",
	"http://10.100.0.12:5002",
	"http://10.100.0.13:5002",
}

func newInsecureClient(baseURL string) (*client.Client, error) {
	return client.New(&client.Config{
		BaseURL: baseURL,
	})
}

func newHANodeClients() ([]*client.Client, error) {
	clients := make([]*client.Client, 0, len(haNodeURLs))
	for _, u := range haNodeURLs {
		c, err := newInsecureClient(u)
		if err != nil {
			return nil, fmt.Errorf("client for %s: %w", u, err)
		}

		clients = append(clients, c)
	}

	return clients, nil
}

// waitForClusterReady polls GetStatus (unauthenticated) on every client
// until all nodes are reachable and exactly one is the leader.
func waitForClusterReady(ctx context.Context, clients []*client.Client) error {
	timeout := 3 * time.Minute
	deadline := time.Now().Add(timeout)
	expected := len(clients)

	for time.Now().Before(deadline) {
		reachable := 0
		leaders := 0
		withLeaderAddr := 0

		for _, c := range clients {
			status, err := c.GetStatus(ctx)
			if err != nil {
				break
			}

			if status.Cluster == nil {
				break
			}

			reachable++

			if status.Cluster.Role == "Leader" {
				leaders++
			}

			if status.Cluster.LeaderAddress != "" {
				withLeaderAddr++
			}
		}

		if reachable == expected && leaders == 1 && withLeaderAddr == expected {
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("cluster not ready after %v: expected %d members with a leader", timeout, expected)
}

// findLeader returns the index and client of the current leader node.
func findLeader(ctx context.Context, clients []*client.Client) (int, *client.Client, error) {
	for i, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			continue
		}

		if status.Cluster != nil && status.Cluster.Role == "Leader" {
			return i, c, nil
		}
	}

	return -1, nil, fmt.Errorf("no leader found")
}

// waitForNewLeader polls the given clients until exactly one reports itself as
// leader. It is used after stopping the old leader to wait for re-election.
func waitForNewLeader(ctx context.Context, clients []*client.Client) (*client.Client, error) {
	timeout := 90 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		for _, c := range clients {
			status, err := c.GetStatus(ctx)
			if err != nil {
				continue
			}

			if status.Cluster != nil && status.Cluster.Role == "Leader" {
				return c, nil
			}
		}

		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("no new leader elected within %v", timeout)
}

// waitForNodeReady polls a single node until it is reachable and reports Ready.
func waitForNodeReady(ctx context.Context, c *client.Client) error {
	timeout := 2 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status, err := c.GetStatus(ctx)
		if err == nil && status.Ready {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("node not ready after %v", timeout)
}

// initializeCluster creates the admin user and API token on the leader,
// then sets the token on all clients.
func initializeCluster(ctx context.Context, leader *client.Client, allClients []*client.Client) error {
	err := leader.Initialize(ctx, &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	resp, err := leader.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{
		Name:   "ha-integration-test",
		Expiry: "",
	})
	if err != nil {
		return fmt.Errorf("create API token: %w", err)
	}

	for _, c := range allClients {
		c.SetToken(resp.Token)
	}

	return nil
}

// waitForAllNodesReady polls GetStatus on every node until all report Ready.
// Ready becomes true after a node completes its full startup (Phase B upgrade),
// meaning it can serve the full API.
func waitForAllNodesReady(ctx context.Context, clients []*client.Client) error {
	timeout := 2 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		allReady := true

		for _, c := range clients {
			status, err := c.GetStatus(ctx)
			if err != nil || !status.Ready {
				allReady = false
				break
			}
		}

		if allReady {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("not all nodes ready after %v", timeout)
}

// waitForFollowerConvergence polls each follower's AppliedIndex until it
// reaches at least minIndex. This ensures Raft replication has delivered
// all committed entries before reading from followers.
func waitForFollowerConvergence(ctx context.Context, clients []*client.Client, minIndex uint64) error {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		converged := true

		for _, c := range clients {
			status, err := c.GetStatus(ctx)
			if err != nil {
				converged = false
				break
			}

			if status.Cluster == nil {
				converged = false
				break
			}

			if status.Cluster.Role == "Leader" {
				continue
			}

			if status.Cluster.Role != "Follower" || status.Cluster.AppliedIndex < minIndex || !status.Ready {
				converged = false
				break
			}
		}

		if converged {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("followers did not converge to index %d within %v", minIndex, timeout)
}

// leaderAppliedIndex returns the current applied Raft index from the leader.
func leaderAppliedIndex(ctx context.Context, leader *client.Client) (uint64, error) {
	status, err := leader.GetStatus(ctx)
	if err != nil {
		return 0, fmt.Errorf("get leader status: %w", err)
	}

	if status.Cluster == nil {
		return 0, fmt.Errorf("leader has no cluster status")
	}

	return status.Cluster.AppliedIndex, nil
}

// waitForMemberSuffrage polls ListClusterMembers until the given nodeID
// appears with the expected suffrage value (e.g. "nonvoter" or "voter").
func waitForMemberSuffrage(ctx context.Context, c *client.Client, nodeID int, wantSuffrage string) error {
	timeout := 2 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		members, err := c.ListClusterMembers(ctx)
		if err == nil {
			for _, m := range members {
				if m.NodeID == nodeID && m.Suffrage == wantSuffrage {
					return nil
				}
			}
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("node %d did not reach suffrage %q within %v", nodeID, wantSuffrage, timeout)
}

// dumpClusterDiagnostics logs node status and cluster members from each
// reachable node. Call from t.Cleanup to aid failure triage.
func dumpClusterDiagnostics(ctx context.Context, dc *DockerClient, clients []*client.Client, logf func(string, ...any)) {
	for i, svc := range haNodeServices {
		logs, err := dc.ComposeLogs(ctx, haComposeDir, svc)
		if err != nil {
			logf("failed to collect logs for %s: %v", svc, err)
		} else {
			logf("=== %s logs ===\n%s", svc, logs)
		}

		if i < len(clients) {
			status, err := clients[i].GetStatus(ctx)
			if err != nil {
				logf("%s status: unreachable (%v)", svc, err)
			} else {
				role := "standalone"
				if status.Cluster != nil {
					role = status.Cluster.Role
				}

				logf("%s status: role=%s initialized=%v", svc, role, status.Initialized)
			}
		}
	}

	for i, c := range clients {
		members, err := c.ListClusterMembers(ctx)
		if err != nil {
			continue
		}

		logf("cluster members (from node %d):", i+1)

		for _, m := range members {
			logf("  node=%d raft=%s api=%s suffrage=%s", m.NodeID, m.RaftAddress, m.APIAddress, m.Suffrage)
		}

		break
	}
}
