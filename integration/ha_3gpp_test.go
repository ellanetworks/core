package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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

		primaryService = "ella-core-1"

		primaryAPIURL   = "http://10.100.0.11:5002"
		secondaryAPIURL = "http://10.100.0.12:5002"
		tertiaryAPIURL  = "http://10.100.0.13:5002"

		primaryN2   = "10.100.0.11:38412"
		secondaryN2 = "10.100.0.12:38412"
		tertiaryN2  = "10.100.0.13:38412"

		gnbN2 = "10.100.0.20"
		gnbN3 = "10.3.0.20"
	)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	adminToken, err := bringUpHA3GPPCluster(ctx, dc, composeDir, composeFile)
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	t.Cleanup(func() {
		for _, svc := range []string{"ella-core-1", "ella-core-2", "ella-core-3"} {
			if logs, logErr := dc.ComposeLogs(ctx, composeDir, svc); logErr == nil {
				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		dc.ComposeDownWithFile(context.Background(), composeDir, composeFile)
	})

	haClient, err := client.New(&client.Config{
		BaseURLs: []string{primaryAPIURL, secondaryAPIURL, tertiaryAPIURL},
	})
	if err != nil {
		t.Fatalf("ella HA client: %v", err)
	}

	haClient.SetToken(adminToken)

	if err := configureNATAndRoute(ctx, haClient); err != nil {
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

	spec := sc.Fixture()
	fx.Apply(spec)

	testerContainer, err := dc.ResolveComposeContainer(ctx, "ha-5g", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	// Kick off the scenario. Stdout is mirrored to the test log AND
	// scanned for the PHASE1_DONE marker so we can synchronise the kill.
	markerCh := make(chan struct{})

	writer := newMarkerWriter(t, "PHASE1_DONE", markerCh)

	argv := []string{
		"core-tester", "run", "ha/failover_connectivity",
		"--ella-core-n2-address", primaryN2,
		"--ella-core-n2-address", secondaryN2,
		"--ella-core-n2-address", tertiaryN2,
		"--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s", gnbN2, gnbN3),
		"--verbose",
	}

	scenarioErr := make(chan error, 1)

	go func() {
		_, execErr := dc.Exec(ctx, testerContainer, argv, false, 5*time.Minute, writer)
		scenarioErr <- execErr
	}()

	select {
	case <-markerCh:
		t.Logf("phase 1 complete; killing %s", primaryService)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for phase-1 marker: %v", ctx.Err())
	case runErr := <-scenarioErr:
		t.Fatalf("scenario exited before phase-1 marker: %v", runErr)
	}

	// Stop the primary core. Docker sends SIGTERM with a grace period;
	// the SCTP association closes cleanly, the gNB's receiver sees EOF,
	// and the gNB promotes peer[1] (secondaryN2). NAT is enabled on
	// Ella Core, so N6 return traffic to the new UPF is unicast-routed
	// via the existing NAT flow — no router-route update needed.
	if err := dc.ComposeStopWithFile(ctx, composeDir, composeFile, primaryService); err != nil {
		t.Fatalf("stop %s: %v", primaryService, err)
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
//  5. Wait for the full cluster to converge (1 leader + N-1 followers).
//  6. Start the tester sidecar and the N6 router (no cluster
//     dependency on either).
//
// Returns the admin token so callers can set it on an HA-mode client.
func bringUpHA3GPPCluster(ctx context.Context, dc *DockerClient, composeDir, composeFile string) (string, error) {
	nodeServices := []string{"ella-core-1", "ella-core-2", "ella-core-3"}

	peers := []string{
		"10.100.0.11:7000",
		"10.100.0.12:7000",
		"10.100.0.13:7000",
	}

	dc.ComposeCleanup(ctx)

	if err := writeHA3GPPNodeConfig(composeDir, 1, peers, ""); err != nil {
		return "", err
	}

	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, nodeServices[0]); err != nil {
		return "", fmt.Errorf("start node 1: %w", err)
	}

	node1URL := "http://10.100.0.11:5002"

	node1, err := client.New(&client.Config{BaseURL: node1URL})
	if err != nil {
		return "", fmt.Errorf("node 1 client: %w", err)
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		return "", fmt.Errorf("node 1 never became ready: %w", err)
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		return "", err
	}

	node1.SetToken(adminToken)

	for i := 1; i < len(nodeServices); i++ {
		nodeID := i + 1

		tok, err := node1.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
			NodeID:     nodeID,
			TTLSeconds: 600,
		})
		if err != nil {
			return "", fmt.Errorf("mint join token for node %d: %w", nodeID, err)
		}

		if err := writeHA3GPPNodeConfig(composeDir, nodeID, peers, tok.Token); err != nil {
			return "", err
		}

		if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, nodeServices[i]); err != nil {
			return "", fmt.Errorf("start node %d: %w", nodeID, err)
		}
	}

	clients := []*client.Client{node1}

	for _, url := range []string{"http://10.100.0.12:5002", "http://10.100.0.13:5002"} {
		c, err := client.New(&client.Config{BaseURL: url})
		if err != nil {
			return "", fmt.Errorf("client for %s: %w", url, err)
		}

		c.SetToken(adminToken)
		clients = append(clients, c)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		return "", fmt.Errorf("cluster not ready: %w", err)
	}

	// Start the tester and router last; they don't affect cluster formation.
	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, "ella-core-tester", "router"); err != nil {
		return "", fmt.Errorf("start tester + router: %w", err)
	}

	return adminToken, nil
}

// writeHA3GPPNodeConfig renders a per-node core.yaml with the ha-5g
// topology interface shape (separate n2 on cluster bridge, n3 on its own
// bridge, n6 by interface name for router egress).
//
// Mirrors the pattern of writeNodeConfig in ha_helpers_test.go but with
// per-node n3 addresses instead of the single-bridge shape used by the
// non-5G HA tests.
func writeHA3GPPNodeConfig(composeDir string, nodeID int, peers []string, joinToken string) error {
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
%s%s`, clusterAddr, n3Addr, clusterAddr, nodeID, clusterAddr, peersYAML.String(), joinTokenLine)

	return os.WriteFile(filepath.Join(cfgDir, "core.yaml"), []byte(body), 0o644)
}

// configureNATAndRoute applies the cluster-wide networking config for
// 5G data plane: NAT on, plus a route for the ping destination via the
// N6 router. Writes go through the HA client so they land on whichever
// node is currently the leader.
func configureNATAndRoute(ctx context.Context, c *client.Client) error {
	if err := c.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: true}); err != nil {
		return fmt.Errorf("update NAT: %w", err)
	}

	if err := c.CreateRoute(ctx, &client.CreateRouteOptions{
		Destination: "8.8.8.8/32",
		Gateway:     "10.6.0.3",
		Interface:   "n6",
		Metric:      0,
	}); err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create route: %w", err)
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
