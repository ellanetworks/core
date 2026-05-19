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

// captureClusterLogs emits each service's container logs via t.Logf
// and, if HA_CLUSTER_LOG_DIR is set, also writes them to
// <dir>/<test-name>/<service>.log for CI artifact upload. Safe to call
// with no clients (e.g. on bring-up failure).
func captureClusterLogs(t *testing.T, dc *DockerClient, composeDir string, services []string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var diskDir string

	if root := os.Getenv("HA_CLUSTER_LOG_DIR"); root != "" {
		diskDir = filepath.Join(root, sanitizeTestName(t.Name()))
		if err := os.MkdirAll(diskDir, 0o755); err != nil {
			t.Logf("captureClusterLogs: mkdir %s: %v", diskDir, err)

			diskDir = ""
		}
	}

	for _, svc := range services {
		logs, err := dc.ComposeLogs(ctx, composeDir, svc)
		if err != nil {
			t.Logf("=== %s logs: collection failed: %v ===", svc, err)
			continue
		}

		t.Logf("=== %s logs ===\n%s", svc, logs)

		if diskDir != "" {
			path := filepath.Join(diskDir, svc+".log")
			if err := os.WriteFile(path, []byte(logs), 0o644); err != nil {
				t.Logf("captureClusterLogs: write %s: %v", path, err)
			}
		}
	}
}

func sanitizeTestName(name string) string {
	return strings.NewReplacer("/", "_", " ", "_").Replace(name)
}

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
func bringUpHACluster(t *testing.T, ctx context.Context, dc *DockerClient) ([]*client.Client, error) {
	return bringUpHAClusterAt(t, ctx, dc, haComposeDir, haNodeServices, nil)
}

// bringUpHAClusterAt brings up a 3-node HA cluster against composeDir.
// The compose file is expected to bind-mount `./cfg/node<n>/core.yaml`
// into each service as /cfg/core.yaml. This helper writes those files.
// extraPeers lets callers with a larger peers list (scaleup) include
// more addresses than the starting set of services.
//
// On any error return, captureClusterLogs is invoked so per-node container
// logs are emitted (and persisted to HA_CLUSTER_LOG_DIR if set) BEFORE the
// next test's ComposeCleanup tears the containers down.
func bringUpHAClusterAt(t *testing.T, ctx context.Context, dc *DockerClient, composeDir string, services []string, extraPeers []string) ([]*client.Client, error) {
	t.Helper()

	dc.ComposeCleanup(ctx)

	fail := func(err error) ([]*client.Client, error) {
		captureClusterLogs(t, dc, composeDir, services)
		return nil, err
	}

	peers := []string{ClusterAddressWithPort(1, 7000), ClusterAddressWithPort(2, 7000), ClusterAddressWithPort(3, 7000)}
	peers = append(peers, extraPeers...)

	// Write node 1's config (no join-token, default voter suffrage).
	if err := writeNodeConfig(composeDir, 1, peers, "", ""); err != nil {
		return fail(err)
	}

	composeFile := ComposeFile()

	// ComposeCleanup above wiped all containers, so node 1 needs to be
	// (re-)created — `up -d` is the only path that does both create and
	// start. A plain `start` would always fail with "no such container".
	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, services[0]); err != nil {
		return fail(fmt.Errorf("start node 1: %w", err))
	}

	node1, err := newInsecureClient(getHANodeURLs()[0])
	if err != nil {
		return fail(err)
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		return fail(fmt.Errorf("node 1 never became ready: %w", err))
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		return fail(err)
	}

	node1.SetToken(adminToken)

	// For each additional node: mint token, write config, start.
	for i := 1; i < len(services); i++ {
		nodeID := i + 1

		if err := stageAndStartJoiner(ctx, dc, node1, composeDir, services[i], nodeID, peers, ""); err != nil {
			return fail(err)
		}
	}

	clients, err := newHANodeClients()
	if err != nil {
		return fail(err)
	}

	for _, c := range clients {
		c.SetToken(adminToken)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		return fail(fmt.Errorf("cluster not ready: %w", err))
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
		Name:      "ha-integration-test",
		ExpiresAt: "",
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
	return writeNodeConfigOpts(composeDir, nodeID, peers, joinToken, initialSuffrage, false)
}

// writeNodeConfigOpts is writeNodeConfig with a useFQDN switch that makes
// cluster.bind-address resolve via Docker's embedded DNS (the compose
// service name) instead of the per-node IP. The FQDN path is what real
// orchestrator-managed deployments use.
func writeNodeConfigOpts(composeDir string, nodeID int, peers []string, joinToken, initialSuffrage string, useFQDN bool) error {
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

	bindHost := ClusterAddressWithBrackets(nodeID)
	if useFQDN {
		bindHost = fmt.Sprintf("ella-core-%d", nodeID)
	}

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
  bind-address: "%s:7000"
  peers:
%s%s%s`, addr, addr, nodeID, bindHost, peersYAML.String(), joinTokenLine, suffrageLine)

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
	return waitForClusterReadyWithin(ctx, clients, 3*time.Minute)
}

// waitForClusterReadyWithin is waitForClusterReady with a caller-supplied
// timeout. Useful for tests that need a tight deadline to distinguish
// "converges quickly" from "converges after crashloop / many retries".
func waitForClusterReadyWithin(ctx context.Context, clients []*client.Client, timeout time.Duration) error {
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
//
// scale-up join target); the helper is intentionally general so future
// tests targeting other node IDs don't need a parallel helper.
//
//nolint:unparam // nodeID happens to be 4 for every existing caller (the
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

// dumpClusterDiagnostics logs container output, per-node status, and
// cluster_members from each reachable node. composeDir MUST match the
// compose project the test brought up — passing the wrong dir silently
// returns empty logs.
//
// (currently skipped) passes haRollingComposeDir.
//
//nolint:unparam // composeDir varies by caller; the rolling-upgrade test
func dumpClusterDiagnostics(t *testing.T, ctx context.Context, dc *DockerClient, composeDir string, services []string, clients []*client.Client) {
	t.Helper()

	captureClusterLogs(t, dc, composeDir, services)

	for i, svc := range services {
		if i >= len(clients) {
			break
		}

		status, err := clients[i].GetStatus(ctx)
		if err != nil {
			t.Logf("%s status: unreachable (%v)", svc, err)
			continue
		}

		role := "standalone"

		var (
			appliedSchema int
			pending       string
		)

		if status.Cluster != nil {
			role = status.Cluster.Role
			appliedSchema = status.Cluster.AppliedSchemaVersion

			if p := status.Cluster.PendingMigration; p != nil {
				pending = fmt.Sprintf(" pending={current=%d target=%d laggard=%d}",
					p.CurrentSchema, p.TargetSchema, p.LaggardNodeId)
			}
		}

		t.Logf("%s status: role=%s initialized=%v ready=%v binarySchema=%d appliedSchema=%d%s",
			svc, role, status.Initialized, status.Ready,
			status.SchemaVersion, appliedSchema, pending)
	}

	for i, c := range clients {
		members, err := c.ListClusterMembers(ctx)
		if err != nil {
			t.Logf("cluster members (from node %d): unreachable (%v)", i+1, err)
			continue
		}

		t.Logf("cluster members (from node %d):", i+1)

		for _, m := range members {
			t.Logf("  node=%d raft=%s api=%s suffrage=%s isLeader=%v binaryVersion=%q drainState=%s",
				m.NodeID, m.RaftAddress, m.APIAddress, m.Suffrage, m.IsLeader,
				m.BinaryVersion, m.DrainState)
		}
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

// fqdnPeers returns the cluster.peers list as compose-service-name FQDNs
// resolved by Docker's embedded DNS — mirrors the orchestrator-managed
// deployment pattern where peers reference stable DNS names rather than
// IPs.
func fqdnPeers(services []string) []string {
	peers := make([]string, 0, len(services))
	for _, s := range services {
		peers = append(peers, fmt.Sprintf("%s:7000", s))
	}

	return peers
}

// fqdnHANodeURLs returns API URLs via the host port map. Cluster-network
// container IPs are dynamic in the FQDN compose, so the test reaches each
// node through compose's published ports on the loopback interface.
func fqdnHANodeURLs() []string {
	return []string{
		"http://127.0.0.1:5002",
		"http://127.0.0.1:5003",
		"http://127.0.0.1:5004",
	}
}

// writeFQDNNodeConfig renders a core.yaml that addresses peers and the
// local bind-address by FQDN, and binds the API/N2 listeners on
// 0.0.0.0 since the cluster-network IP is assigned dynamically and is
// not known at config-write time.
func writeFQDNNodeConfig(composeDir string, nodeID int, peers []string, joinToken string) error {
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

	var peersYAML strings.Builder

	for _, p := range peers {
		fmt.Fprintf(&peersYAML, "      - %q\n", p)
	}

	joinTokenLine := ""
	if joinToken != "" {
		joinTokenLine = fmt.Sprintf("  join-token: %q\n", joinToken)
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
    address: "0.0.0.0"
    port: 38412
  n3:
    name: "eth0"
  n6:
    name: "n6"
  api:
    address: "0.0.0.0"
    port: 5002
xdp:
  attach-mode: "generic"
cluster:
  enabled: true
  node-id: %d
  bind-address: "ella-core-%d:7000"
  peers:
%s%s`, nodeID, nodeID, peersYAML.String(), joinTokenLine)

	return os.WriteFile(filepath.Join(cfgDir, "core.yaml"), []byte(body), 0o644)
}

// bringUpHAFQDNClusterAt is bringUpHAClusterAt with FQDN-only cluster
// addressing. Peers and bind-address reference the compose service name
// instead of a pinned IP, and the compose file is the caller's choice
// (typically one without ipv4_address pins on the cluster network).
func bringUpHAFQDNClusterAt(t *testing.T, ctx context.Context, dc *DockerClient, composeDir, composeFile string, services []string) ([]*client.Client, error) {
	t.Helper()

	dc.ComposeCleanup(ctx)

	fail := func(err error) ([]*client.Client, error) {
		captureClusterLogs(t, dc, composeDir, services)
		return nil, err
	}

	peers := fqdnPeers(services)
	urls := fqdnHANodeURLs()

	if err := writeFQDNNodeConfig(composeDir, 1, peers, ""); err != nil {
		return fail(err)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, services[0]); err != nil {
		return fail(fmt.Errorf("start node 1: %w", err))
	}

	node1, err := newInsecureClient(urls[0])
	if err != nil {
		return fail(err)
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		return fail(fmt.Errorf("node 1 never became ready: %w", err))
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		return fail(err)
	}

	node1.SetToken(adminToken)

	for i := 1; i < len(services); i++ {
		nodeID := i + 1

		if err := stageAndStartFQDNJoiner(ctx, dc, node1, composeDir, composeFile, services[i], nodeID, peers); err != nil {
			return fail(err)
		}
	}

	clients := make([]*client.Client, 0, len(urls))

	for _, u := range urls {
		c, err := newInsecureClient(u)
		if err != nil {
			return fail(fmt.Errorf("client for %s: %w", u, err))
		}

		c.SetToken(adminToken)
		clients = append(clients, c)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		return fail(fmt.Errorf("cluster not ready: %w", err))
	}

	return clients, nil
}

func stageAndStartFQDNJoiner(ctx context.Context, dc *DockerClient, leader *client.Client, composeDir, composeFile, service string, nodeID int, peers []string) error {
	tok, err := leader.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     nodeID,
		TTLSeconds: 600,
	})
	if err != nil {
		return fmt.Errorf("mint join token for node %d: %w", nodeID, err)
	}

	if err := writeFQDNNodeConfig(composeDir, nodeID, peers, tok.Token); err != nil {
		return err
	}

	return dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, service)
}
