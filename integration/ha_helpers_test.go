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
// until all expectedMembers nodes are reachable and exactly one is the leader.
func waitForClusterReady(ctx context.Context, clients []*client.Client, expectedMembers int) error {
	timeout := 3 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		reachable := 0
		leaders := 0

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
		}

		if reachable == expectedMembers && leaders == 1 {
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("cluster not ready after %v: expected %d members with a leader", timeout, expectedMembers)
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
