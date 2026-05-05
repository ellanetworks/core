package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	// Side-effect import to register the ha/failover_connectivity scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration3GPPHAFailover brings up a 3-node Raft cluster plus a
// core-tester sidecar, exercises registration + connectivity against the
// primary core, kills the primary, and exercises registration +
// connectivity against the surviving cluster.
//
// Lives in its own workflow (integration-tests-ha3gpp.yaml) because it
// needs both the ella-core and ella-core-tester images; the
// integration-tests-ha.yaml workflow loads only ella-core. The test
// function is named so it does NOT match the `-run TestIntegrationHA`
// filter used by integration-tests-ha.yaml.
//
// Passes only if the ha/failover_connectivity scenario exits 0.
func TestIntegration3GPPHAFailover(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	const (
		composeDir  = "compose/ha-5g/"
		composeFile = "compose.yaml"

		gnbN2 = "10.100.0.20"
		gnbN3 = "10.3.0.20"
	)

	// Per-node addresses, 0-indexed (nodeX = index X-1).
	var (
		nodeServices = [3]string{"ella-core-1", "ella-core-2", "ella-core-3"}
		nodeAPIURLs  = [3]string{
			"http://10.100.0.11:5002",
			"http://10.100.0.12:5002",
			"http://10.100.0.13:5002",
		}
		nodeN2Addrs = [3]string{
			"10.100.0.11:38412",
			"10.100.0.12:38412",
			"10.100.0.13:38412",
		}
	)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	adminToken, nodeClients, err := bringUpHA3GPPCluster(t, ctx, dc, composeDir, composeFile, bringUpHA3GPPClusterOpts{
		// Exercise the hostname-resolved peer path; TestIntegration3GPPMultiGNB
		// covers the IP-literal path.
		UseFQDN:       true,
		ExtraServices: []string{"ella-core-tester", "router"},
	})
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	t.Cleanup(func() {
		// Use a fresh context that outlives the test body. The test's
		// `ctx` is cancelled by `defer cancel()` when the test function
		// unwinds (including on t.Fatalf), which would otherwise make
		// every ComposeLogs call fail immediately with context.Canceled
		// and silently skip log collection.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		for i, svc := range nodeServices {
			logs, logErr := dc.ComposeLogs(cleanupCtx, composeDir, svc)
			if logErr != nil {
				t.Logf("=== %s logs: collection failed: %v ===", svc, logErr)
			} else {
				t.Logf("=== %s logs ===\n%s", svc, logs)
			}

			if i < len(nodeClients) {
				status, statusErr := nodeClients[i].GetStatus(cleanupCtx)
				if statusErr != nil {
					t.Logf("%s status: unreachable (%v)", svc, statusErr)
				} else {
					role := "standalone"
					if status.Cluster != nil {
						role = status.Cluster.Role
					}

					t.Logf("%s status: role=%s initialized=%v ready=%v",
						svc, role, status.Initialized, status.Ready)
				}
			}
		}

		for i, c := range nodeClients {
			members, err := c.ListClusterMembers(cleanupCtx)
			if err != nil {
				continue
			}

			t.Logf("cluster members (from node %d):", i+1)

			for _, m := range members {
				t.Logf("  node=%d raft=%s api=%s suffrage=%s isLeader=%v",
					m.NodeID, m.RaftAddress, m.APIAddress, m.Suffrage, m.IsLeader)
			}

			break
		}

		dc.ComposeDownWithFile(cleanupCtx, composeDir, composeFile)
	})

	haClient, err := client.New(&client.Config{
		BaseURLs: nodeAPIURLs[:],
	})
	if err != nil {
		t.Fatalf("ella HA client: %v", err)
	}

	haClient.SetToken(adminToken)

	if err := configureNATAndRoute(ctx, nodeClients); err != nil {
		t.Fatalf("configure NAT + route: %v", err)
	}

	fx := fixture.New(t, ctx, haClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	// The failover scenario declares its own Fixture() (the default
	// subscriber). Apply it via the spec so this test stays aligned with
	// the scenario's declared needs.
	sc, ok := scenarios.Get("ha/failover_connectivity")
	if !ok {
		t.Fatalf("scenario ha/failover_connectivity not registered")
	}

	scenariosEnv := scenarios.Env{
		CoreN2Addresses: nodeN2Addrs[:],
		GNBs: []scenarios.GNB{
			{Name: "gnb1", N2Address: gnbN2, N3Address: gnbN3},
		},
	}
	spec := sc.Fixture(scenariosEnv)
	fx.Apply(spec)

	testerContainer, err := dc.ResolveComposeContainer(ctx, "ha-5g", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	// Order the gNB's core list so the current raft leader is the primary.
	// Killing the leader then is the interesting HA signal — it exercises
	// both raft re-election AND gNB SCTP failover in one pass.
	leaderIdx, _, err := findLeader(ctx, nodeClients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	leaderService := nodeServices[leaderIdx]
	orderedN2 := orderLeaderFirst(nodeN2Addrs[:], leaderIdx)

	t.Logf("leader is %s; gNB primary N2 = %s", leaderService, orderedN2[0])

	// Kick off the scenario. Stdout is mirrored to the test log AND
	// scanned for the PHASE1_DONE marker so we can synchronise the kill.
	markerCh := make(chan struct{})

	writer := newMarkerWriter(t, "PHASE1_DONE", markerCh)

	argv := []string{"core-tester", "run", "ha/failover_connectivity"}
	for _, addr := range orderedN2 {
		argv = append(argv, "--ella-core-n2-address", addr)
	}

	argv = append(argv,
		"--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s", gnbN2, gnbN3),
		"--verbose",
	)

	scenarioErr := make(chan error, 1)

	go func() {
		_, execErr := dc.Exec(ctx, testerContainer, argv, false, 5*time.Minute, writer)
		scenarioErr <- execErr
	}()

	select {
	case <-markerCh:
		t.Logf("phase 1 complete; killing leader %s", leaderService)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for phase-1 marker: %v", ctx.Err())
	case runErr := <-scenarioErr:
		t.Fatalf("scenario exited before phase-1 marker: %v", runErr)
	}

	// Kill the leader with SIGKILL instead of a graceful SIGTERM. A
	// graceful `docker compose stop` relies on Core finishing its
	// shutdown sequence (conn.Close on every SCTP association) before
	// the process exits and docker reaps the container's network
	// namespace. In practice the SCTP SHUTDOWN handshake doesn't
	// complete before Core exits — the tester is stuck in
	// SHUTDOWN_RECEIVED until kernel heartbeat timeouts fire (minutes).
	// `docker kill` sends SIGKILL directly, which makes the kernel emit
	// an SCTP ABORT (no handshake needed) as it reaps Core's sockets.
	// The tester's blocked SCTPRead unblocks with io.EOF within ms,
	// which drives the failover path.
	if err := composeKill(ctx, composeDir, composeFile, leaderService); err != nil {
		t.Fatalf("kill %s: %v", leaderService, err)
	}

	select {
	case runErr := <-scenarioErr:
		if runErr != nil {
			t.Fatalf("scenario failed: %v", runErr)
		}
	case <-ctx.Done():
		t.Fatalf("scenario did not exit: %v", ctx.Err())
	}

	t.Log("failover scenario passed both phases")
}

// bringUpHA3GPPCluster stages a 3-node HA cluster specifically for this
// test's ha-5g compose topology. Flow:
//
//  1. Write node 1's config (no join-token). Start only ella-core-1.
//  2. Wait for node 1 to become ready (unauthenticated GetStatus).
//  3. Initialize the admin user; mint an API token.
//  4. For nodes 2 and 3: mint a cluster join token via node 1, write
//     the node's config with the token embedded, start the service.
//  5. Wait for the full cluster to converge (1 leader + N-1 followers)
//     AND every node to report Ready — the API is not fully usable
//     until Phase B startup completes.
//  6. Start the tester sidecar and the N6 router (no cluster
//     dependency on either).
//
// Returns the admin token plus a per-node client slice (with the admin
// token set on each) so callers can use findLeader / waitForAutopilotHealthy
// etc.
// bringUpHA3GPPCluster brings up a 3-node Ella Core cluster from
// composeDir/composeFile and, after the cluster is converged, starts
// any extraServices listed (typically the tester sidecar and the N6
// router). Compose topologies that need a different sidecar shape
// (e.g., one tester per gNB) pass their own service names instead.
// bringUpHA3GPPClusterOpts captures the optional flags accepted by
// bringUpHA3GPPCluster. Anything that varies per-test goes here so the
// helper signature stays stable.
type bringUpHA3GPPClusterOpts struct {
	// UseFQDN selects compose-service-name FQDNs (ella-core-1 / ella-core-2
	// / ella-core-3) for the cluster peer list and bind-address instead of
	// raw IP literals. Docker's embedded DNS on the user-defined `cluster`
	// network resolves those names to the per-node IP. Used to verify that
	// the cluster control-plane works against hostname peers — the way
	// real deployments name their members. Default false: peers stay on
	// IP literals so most tests still cover the IP-based path.
	UseFQDN bool

	// ExtraServices are compose services started after the cluster is
	// converged (testers, routers). Cluster formation does not depend on
	// them.
	ExtraServices []string
}

func bringUpHA3GPPCluster(t *testing.T, ctx context.Context, dc *DockerClient, composeDir, composeFile string, opts bringUpHA3GPPClusterOpts) (string, []*client.Client, error) {
	t.Helper()

	nodeServices := []string{"ella-core-1", "ella-core-2", "ella-core-3"}

	var peers []string

	if opts.UseFQDN {
		peers = []string{
			"ella-core-1:7000",
			"ella-core-2:7000",
			"ella-core-3:7000",
		}
	} else {
		peers = []string{
			"10.100.0.11:7000",
			"10.100.0.12:7000",
			"10.100.0.13:7000",
		}
	}

	dc.ComposeCleanup(ctx)

	fail := func(err error) (string, []*client.Client, error) {
		captureClusterLogs(t, dc, composeDir, nodeServices)
		return "", nil, err
	}

	if err := writeHA3GPPNodeConfig(composeDir, 1, peers, "", opts.UseFQDN); err != nil {
		return fail(err)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, nodeServices[0]); err != nil {
		return fail(fmt.Errorf("start node 1: %w", err))
	}

	node1URL := "http://10.100.0.11:5002"

	node1, err := client.New(&client.Config{BaseURL: node1URL})
	if err != nil {
		return fail(fmt.Errorf("node 1 client: %w", err))
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		return fail(fmt.Errorf("node 1 never became ready: %w", err))
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		return fail(err)
	}

	node1.SetToken(adminToken)

	for i := 1; i < len(nodeServices); i++ {
		nodeID := i + 1

		tok, err := node1.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
			NodeID:     nodeID,
			TTLSeconds: 600,
		})
		if err != nil {
			return fail(fmt.Errorf("mint join token for node %d: %w", nodeID, err))
		}

		if err := writeHA3GPPNodeConfig(composeDir, nodeID, peers, tok.Token, opts.UseFQDN); err != nil {
			return fail(err)
		}

		if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, nodeServices[i]); err != nil {
			return fail(fmt.Errorf("start node %d: %w", nodeID, err))
		}
	}

	clients := []*client.Client{node1}

	for _, url := range []string{"http://10.100.0.12:5002", "http://10.100.0.13:5002"} {
		c, err := client.New(&client.Config{BaseURL: url})
		if err != nil {
			return fail(fmt.Errorf("client for %s: %w", url, err))
		}

		c.SetToken(adminToken)
		clients = append(clients, c)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		return fail(fmt.Errorf("cluster not ready: %w", err))
	}

	// waitForClusterReady only asserts "reachable + 1 leader elected". The
	// full API (everything behind Phase B startup) requires status.Ready
	// on every node. Without this, the first post-up write — fixture +
	// NAT + route — can race against node startup and silently incur
	// retry-on-503 stalls via the haRequester.
	if err := waitForAllNodesReady(ctx, clients); err != nil {
		return fail(fmt.Errorf("nodes not ready: %w", err))
	}

	// Start any caller-supplied sidecars (testers, router) last; they
	// don't affect cluster formation.
	if len(opts.ExtraServices) > 0 {
		if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, opts.ExtraServices...); err != nil {
			return fail(fmt.Errorf("start extra services %v: %w", opts.ExtraServices, err))
		}
	}

	return adminToken, clients, nil
}

// orderLeaderFirst returns a slice of addresses with leaderIdx moved to
// position 0, preserving the relative order of the rest.
func orderLeaderFirst(addrs []string, leaderIdx int) []string {
	out := make([]string, 0, len(addrs))
	out = append(out, addrs[leaderIdx])

	for i, a := range addrs {
		if i != leaderIdx {
			out = append(out, a)
		}
	}

	return out
}

// composeKill sends SIGKILL to the named service via `docker compose kill`.
// Used by the failover test instead of a graceful stop so the kernel
// emits an SCTP ABORT on Core's sockets (rather than an incomplete
// graceful SHUTDOWN), which reliably wakes the gNB's blocked receiver.
func composeKill(ctx context.Context, composeDir, composeFile, service string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", composeFile,
		"kill",
		service,
	)
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose kill %s: %w", service, err)
	}

	return nil
}

// writeHA3GPPNodeConfig renders a per-node core.yaml with the ha-5g
// topology interface shape (separate n2 on cluster bridge, n3 on its own
// bridge, n6 by interface name for router egress).
//
// Mirrors the pattern of writeNodeConfig in ha_helpers_test.go but with
// per-node n3 addresses instead of the single-bridge shape used by the
// non-5G HA tests.
//
// When useFQDN is true, the cluster.bind-address uses the compose service
// name (ella-core-N) rather than the per-node IP, exercising the
// hostname-resolved peer path. n2/n3/api addresses stay on IP literals
// because they are data-plane endpoints reached by simulators that bind
// directly to fixed IPs.
func writeHA3GPPNodeConfig(composeDir string, nodeID int, peers []string, joinToken string, useFQDN bool) error {
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

	clusterAddr := fmt.Sprintf("10.100.0.%d", 10+nodeID)
	n3Addr := fmt.Sprintf("10.3.0.%d", 10+nodeID)

	clusterBindHost := clusterAddr
	if useFQDN {
		clusterBindHost = fmt.Sprintf("ella-core-%d", nodeID)
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
    address: %q
    port: 38412
  n3:
    address: %q
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
%s%s`, clusterAddr, n3Addr, clusterAddr, nodeID, clusterBindHost, peersYAML.String(), joinTokenLine)

	return os.WriteFile(filepath.Join(cfgDir, "core.yaml"), []byte(body), 0o644)
}

// configureNATAndRoute applies NAT + default route to each node directly.
// These tables are node-scoped in HA mode and are not replicated.
func configureNATAndRoute(ctx context.Context, nodeClients []*client.Client) error {
	family := DetectIPFamily()

	for i, c := range nodeClients {
		if family == IPv4Only || family == DualStack {
			if err := c.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: true}); err != nil {
				return fmt.Errorf("update NAT on node %d: %w", i+1, err)
			}

			if err := c.CreateRoute(ctx, &client.CreateRouteOptions{
				Destination: "8.8.8.8/32",
				Gateway:     "10.6.0.3",
				Interface:   "n6",
				Metric:      0,
			}); err != nil && !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("create route on node %d: %w", i+1, err)
			}
		}

		if family == IPv6Only || family == DualStack {
			if err := c.CreateRoute(ctx, &client.CreateRouteOptions{
				Destination: UeIPv6Pool(),
				Gateway:     N6IPv6Address(),
				Interface:   "n6",
				Metric:      0,
			}); err != nil && !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("create ipv6 route: %w", err)
			}
		}
	}

	return nil
}

// markerWriter mirrors writes to a testing.T log AND watches for a
// substring. When the substring first appears, it closes a channel so
// the orchestrator can synchronise with the scenario.
//
// Subtle: docker exec does not guarantee chunk boundaries align with
// lines. The buffered scan below handles partial lines, so the marker
// can appear split across writes without being missed.
type markerWriter struct {
	t      *testing.T
	marker []byte
	buf    bytes.Buffer
	ch     chan<- struct{}
	once   sync.Once
	mu     sync.Mutex
}

func newMarkerWriter(t *testing.T, marker string, found chan<- struct{}) io.Writer {
	t.Helper()

	return &markerWriter{
		t:      t,
		marker: []byte(marker),
		ch:     found,
	}
}

func (w *markerWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)

	for {
		b := w.buf.Bytes()

		idx := bytes.IndexByte(b, '\n')
		if idx < 0 {
			break
		}

		line := string(b[:idx])
		w.buf.Next(idx + 1)

		w.t.Log(strings.TrimRight(line, "\r"))

		if bytes.Contains([]byte(line), w.marker) {
			w.once.Do(func() { close(w.ch) })
		}
	}

	return len(p), nil
}
