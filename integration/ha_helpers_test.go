package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

const haComposeDir = "compose/ha/"

var haNodeServices = []string{"ella-core-1", "ella-core-2", "ella-core-3"}

// getHANodeURLs returns the API URLs for HA nodes based on the current IP family
func getHANodeURLs() []string {
	urls := make([]string, 3)
	for i := 1; i <= 3; i++ {
		urls[i-1] = APIAddressForCluster(i)
	}

	return urls
}

// bringUpHACluster stages a 3-node HA cluster from scratch against the
// default haComposeDir.
func bringUpHACluster(ctx context.Context, dc *DockerClient) ([]*client.Client, error) {
	return bringUpHAClusterAt(ctx, dc, haComposeDir, haNodeServices, nil)
}

// bringUpHAClusterAt brings up a 3-node HA cluster against composeDir.
// The compose file is expected to bind-mount `./cfg/node<n>/core.yaml`
// into each service as /cfg/core.yaml. This helper writes those files.
// extraPeers lets callers with a larger peers list (scaleup) include
// more addresses than the starting set of services.
func bringUpHAClusterAt(ctx context.Context, dc *DockerClient, composeDir string, services []string, extraPeers []string) ([]*client.Client, error) {
	dc.ComposeCleanup(ctx)

	peers := []string{ClusterAddressWithPort(1, 7000), ClusterAddressWithPort(2, 7000), ClusterAddressWithPort(3, 7000)}
	peers = append(peers, extraPeers...)

	// Write node 1's config (no join-token, default voter suffrage).
	if err := writeNodeConfig(composeDir, 1, peers, "", ""); err != nil {
		return nil, err
	}

	composeFile := ComposeFile()

	if err := dc.ComposeStartWithFile(ctx, composeDir, services[0], composeFile); err != nil {
		// Service may not exist yet — create and start.
		if err2 := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, services[0]); err2 != nil {
			return nil, fmt.Errorf("start node 1: %w (create: %v)", err, err2)
		}
	}

	node1, err := newInsecureClient(getHANodeURLs()[0])
	if err != nil {
		return nil, err
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		return nil, fmt.Errorf("node 1 never became ready: %w", err)
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		return nil, err
	}

	node1.SetToken(adminToken)

	// For each additional node: mint token, write config, start.
	for i := 1; i < len(services); i++ {
		nodeID := i + 1

		if err := stageAndStartJoiner(ctx, dc, node1, composeDir, services[i], nodeID, peers, ""); err != nil {
			return nil, err
		}
	}

	clients, err := newHANodeClients()
	if err != nil {
		return nil, err
	}

	for _, c := range clients {
		c.SetToken(adminToken)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		return nil, fmt.Errorf("cluster not ready: %w", err)
	}

	return clients, nil
}

// initializeAndGetAdminToken creates the first admin user on the leader
// and mints a long-lived API token for driving the test.
func initializeAndGetAdminToken(ctx context.Context, leader *client.Client) (string, error) {
	if err := leader.Initialize(ctx, &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}); err != nil {
		return "", fmt.Errorf("initialize: %w", err)
	}

	resp, err := leader.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{
		Name:   "ha-integration-test",
		Expiry: "",
	})
	if err != nil {
		return "", fmt.Errorf("create API token: %w", err)
	}

	return resp.Token, nil
}

// stageAndStartJoiner mints a join token for nodeID, writes the node's
// core.yaml with the token embedded, and brings the service up. Pass an
// empty initialSuffrage to accept the daemon default ("voter").
func stageAndStartJoiner(ctx context.Context, dc *DockerClient, leader *client.Client, composeDir, service string, nodeID int, peers []string, initialSuffrage string) error {
	tok, err := leader.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     nodeID,
		TTLSeconds: 600,
	})
	if err != nil {
		return fmt.Errorf("mint join token for node %d: %w", nodeID, err)
	}

	if err := writeNodeConfig(composeDir, nodeID, peers, tok.Token, initialSuffrage); err != nil {
		return err
	}

	return dc.ComposeUpServicesWithFile(ctx, composeDir, ComposeFile(), service)
}

// writeNodeConfig renders the node's core.yaml into the compose dir's
// bind-mount path (./cfg/node<n>/core.yaml). Pass an empty
// initialSuffrage to omit the cluster.initial-suffrage field.
func writeNodeConfig(composeDir string, nodeID int, peers []string, joinToken, initialSuffrage string) error {
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

	var peersYAML strings.Builder

	for _, p := range peers {
		fmt.Fprintf(&peersYAML, "      - %q\n", p)
	}

	joinTokenLine := ""
	if joinToken != "" {
		joinTokenLine = fmt.Sprintf("  join-token: %q\n", joinToken)
	}

	suffrageLine := ""
	if initialSuffrage != "" {
		suffrageLine = fmt.Sprintf("  initial-suffrage: %q\n", initialSuffrage)
	}

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
xdp:
  attach-mode: "generic"
cluster:
  enabled: true
  node-id: %d
  bind-address: "%s"
  peers:
%s%s%s`, addr, addr, nodeID, ClusterAddressWithPort(nodeID, 7000), peersYAML.String(), joinTokenLine, suffrageLine)

	return os.WriteFile(filepath.Join(cfgDir, "core.yaml"), []byte(body), 0o644)
}

func newInsecureClient(baseURL string) (*client.Client, error) {
	return client.New(&client.Config{
		BaseURL: baseURL,
	})
}

func newHANodeClients() ([]*client.Client, error) {
	urls := getHANodeURLs()

	clients := make([]*client.Client, 0, len(urls))
	for _, u := range urls {
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

			if status.Cluster.LeaderAPIAddress != "" {
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

// waitForAutopilotHealthy polls GetAutopilotState on the given client until
// the cluster reports healthy with the expected failure tolerance, and every
// listed peer is individually healthy. Used to confirm raft-autopilot has
// caught up after formation or leadership changes.
func waitForAutopilotHealthy(ctx context.Context, c *client.Client, wantFailureTolerance, wantServers int) (*client.AutopilotState, error) {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	var last *client.AutopilotState

	for time.Now().Before(deadline) {
		state, err := c.GetAutopilotState(ctx)
		if err == nil {
			last = state
			if state.Healthy && state.FailureTolerance == wantFailureTolerance && len(state.Servers) == wantServers {
				allHealthy := true

				for _, s := range state.Servers {
					if !s.Healthy {
						allHealthy = false
						break
					}
				}

				if allHealthy {
					return state, nil
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return last, fmt.Errorf("autopilot did not report healthy (failureTolerance=%d, servers=%d) within %v; last=%+v",
		wantFailureTolerance, wantServers, timeout, last)
}

// waitForAutopilotReportsUnhealthy polls autopilot on leader until the given
// node is reported unhealthy. Autopilot flips a peer unhealthy once
// LastContactThreshold (10s) elapses without heartbeats.
func waitForAutopilotReportsUnhealthy(ctx context.Context, leader *client.Client, nodeID int) (*client.AutopilotState, error) {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	var last *client.AutopilotState

	for time.Now().Before(deadline) {
		state, err := leader.GetAutopilotState(ctx)
		if err == nil {
			last = state

			for _, s := range state.Servers {
				if s.NodeID == nodeID && !s.Healthy {
					return state, nil
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	return last, fmt.Errorf("autopilot did not flag node %d unhealthy within %v; last=%+v",
		nodeID, timeout, last)
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

// assertMembershipConsistent fails if clients return different
// cluster_members sets. Polls 10s — callers should already be past
// any settle wait (waitForFollowerConvergence etc.); this catches
// persistent divergence, not apply-path race.
func assertMembershipConsistent(t *testing.T, ctx context.Context, clients []*client.Client) {
	t.Helper()

	const deadline = 10 * time.Second

	end := time.Now().Add(deadline)

	var lastMismatch string

	for {
		snapshots, err := collectMembershipSnapshots(ctx, clients)
		if err == nil {
			diff := membershipDiff(snapshots)
			if diff == "" {
				return
			}

			lastMismatch = diff
		} else {
			lastMismatch = err.Error()
		}

		if !time.Now().Before(end) {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("cluster_members not consistent across nodes after %s: %s", deadline, lastMismatch)
}

func collectMembershipSnapshots(ctx context.Context, clients []*client.Client) ([]string, error) {
	out := make([]string, 0, len(clients))

	for i, c := range clients {
		members, err := c.ListClusterMembers(ctx)
		if err != nil {
			return nil, fmt.Errorf("list members from node %d: %w", i+1, err)
		}

		sort.Slice(members, func(a, b int) bool {
			return members[a].NodeID < members[b].NodeID
		})

		buf, err := json.Marshal(members)
		if err != nil {
			return nil, fmt.Errorf("marshal members from node %d: %w", i+1, err)
		}

		out = append(out, string(buf))
	}

	return out, nil
}

// membershipDiff returns "" on match, otherwise a node 1 vs node N diff.
func membershipDiff(snapshots []string) string {
	if len(snapshots) < 2 {
		return ""
	}

	ref := snapshots[0]

	for i := 1; i < len(snapshots); i++ {
		if snapshots[i] != ref {
			return fmt.Sprintf("node 1 = %s\nnode %d = %s", ref, i+1, snapshots[i])
		}
	}

	return ""
}
