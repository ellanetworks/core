package integration_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// testerEnv owns the core-tester compose lifecycle and exposes helpers to
// invoke scenarios via docker exec.
type testerEnv struct {
	t  *testing.T
	dc *DockerClient

	composeDir  string
	composeFile string

	// Client is an authenticated Ella Core client already bootstrapped
	// (admin user created, API token set, default networking config
	// applied).
	Client *client.Client

	// CoreN2Addresses is passed as --ella-core-n2-address (repeatable).
	// One entry for single-core, three for HA.
	CoreN2Addresses []string

	// GNBs is passed as --gnb (repeatable), one per entry.
	GNBs []testerGNB

	// TesterContainer is the resolved name of the sidecar container
	// hosting core-tester.
	TesterContainer string
}

// testerGNB captures one gNB's addresses as seen from inside the compose
// network.
type testerGNB struct {
	Name        string
	N2Address   string
	N3Address   string
	N3Secondary string
}

// setupTesterEnv brings the core-tester compose up, waits for Ella Core
// to be ready, bootstraps it, and returns the env. Teardown and log
// collection are registered via t.Cleanup.
func setupTesterEnv(ctx context.Context, t *testing.T) *testerEnv {
	t.Helper()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	const composeDir = "compose/core-tester/"

	composeFile := ComposeFile()

	dc.ComposeCleanup(ctx)

	if err := dc.ComposeUpWithFile(ctx, composeDir, composeFile); err != nil {
		t.Fatalf("compose up (%s): %v", composeFile, err)
	}

	t.Cleanup(func() {
		logs, err := dc.ComposeLogs(ctx, composeDir, "ella-core")
		if err == nil {
			t.Logf("=== ella-core container logs ===\n%s", logs)
		}

		dc.ComposeDownWithFile(ctx, composeDir, composeFile)
	})

	cl, err := client.New(&client.Config{BaseURL: APIAddress()})
	if err != nil {
		t.Fatalf("ella client: %v", err)
	}

	if err := waitForEllaCoreReady(ctx, cl); err != nil {
		t.Fatalf("wait for ella core: %v", err)
	}

	if err := bootstrapTesterCore(ctx, cl); err != nil {
		t.Fatalf("bootstrap core: %v", err)
	}

	container, err := dc.ResolveComposeContainer(ctx, "core-tester", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	return &testerEnv{
		t:           t,
		dc:          dc,
		composeDir:  composeDir,
		composeFile: composeFile,
		Client:      cl,
		CoreN2Addresses: []string{
			net.JoinHostPort(N2Address(0), "38412"),
		},
		GNBs: []testerGNB{
			{
				Name:        "gnb1",
				N2Address:   CoreTesterDefaultAddress(),
				N3Address:   CoreTesterN3Address(),
				N3Secondary: CoreTesterN3AddressSecondary(),
			},
		},
		TesterContainer: container,
	}
}

// bootstrapTesterCore initialises Ella Core (admin user, API token) and
// applies the default networking config (NAT + route to 8.8.8.8 via the
// router container). Safe to call on a fresh compose.
func bootstrapTesterCore(ctx context.Context, cl *client.Client) error {
	if err := cl.Initialize(ctx, &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	resp, err := cl.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{
		Name: "integration-test-token",
	})
	if err != nil {
		return fmt.Errorf("create API token: %w", err)
	}

	cl.SetToken(resp.Token)

	if err := cl.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: true}); err != nil {
		return fmt.Errorf("update NAT: %w", err)
	}

	if err := cl.CreateRoute(ctx, &client.CreateRouteOptions{
		Destination: "8.8.8.8/32",
		Gateway:     N6Address(),
		Interface:   "n6",
		Metric:      0,
	}); err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create route: %w", err)
	}

	return nil
}

// RunScenario invokes `core-tester run <scenario>` in the sidecar,
// injecting --ella-core-n2-address and --gnb from the compose topology.
// Fails the subtest on non-zero exit. stdout/stderr mirror to the test
// log.
func (e *testerEnv) RunScenario(ctx context.Context, t *testing.T, scenario string, extraArgs ...string) {
	t.Helper()

	argv := []string{"core-tester", "run", scenario}

	for _, addr := range e.CoreN2Addresses {
		argv = append(argv, "--ella-core-n2-address", addr)
	}

	for _, g := range e.GNBs {
		spec := fmt.Sprintf("%s,n2=%s,n3=%s", g.Name, g.N2Address, g.N3Address)
		if g.N3Secondary != "" {
			spec += ",n3-secondary=" + g.N3Secondary
		}

		argv = append(argv, "--gnb", spec)
	}

	argv = append(argv, "--verbose")
	argv = append(argv, extraArgs...)

	t.Logf("running: %s", strings.Join(argv, " "))

	if _, err := e.dc.Exec(ctx, e.TesterContainer, argv, false, 5*time.Minute, logWriter{t}); err != nil {
		t.Fatalf("scenario %q failed: %v", scenario, err)
	}
}
