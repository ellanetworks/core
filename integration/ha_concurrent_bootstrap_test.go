package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// TestIntegrationHAFreshClusterConcurrentBootstrap brings up a fresh
// 3-node cluster with all nodes started concurrently and FQDN peers
// (resolved via Docker's embedded DNS, mirroring an orchestrator's
// stable per-pod DNS). Each node's peers list includes itself, the
// natural shape when one config template ships to every replica.
//
// Phase A starts node 1 alone to mint join tokens for nodes 2 and 3.
// Phase B stops node 1 and re-creates all three with a single
// compose-up so they race to bind their listeners while dialing each
// other.
//
// compose-no-restart.yaml disables container auto-restart so a
// regression that crashes the joiner surfaces as a stuck cluster
// rather than being papered over by compose retrying the container.
func TestIntegrationHAFreshClusterConcurrentBootstrap(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("test currently only supports IPv4; no compose-no-restart variant exists for ipv6/dualstack")
	}

	const (
		composeDir  = haComposeDir
		composeFile = "compose-no-restart.yaml"
	)

	ctx := context.Background()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dc.Close(); err != nil {
			t.Logf("failed to close docker client: %v", err)
		}
	}()

	fqdnPeers := []string{
		"ella-core-1:7000",
		"ella-core-2:7000",
		"ella-core-3:7000",
	}

	dc.ComposeCleanup(ctx)

	defer func() {
		// Best-effort log capture before the next test tears containers down.
		captureClusterLogs(t, dc, composeDir, haNodeServices)
	}()

	// --- Phase A: founder up, mint tokens for nodes 2 and 3. ---

	t.Log("phase A: starting node 1 as founder")

	if err := writeNodeConfigOpts(composeDir, 1, fqdnPeers, "", "", true); err != nil {
		t.Fatalf("write node 1 config: %v", err)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, "ella-core-1"); err != nil {
		t.Fatalf("start node 1: %v", err)
	}

	node1, err := newInsecureClient(getHANodeURLs()[0])
	if err != nil {
		t.Fatalf("node 1 client: %v", err)
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		t.Fatalf("node 1 never became ready: %v", err)
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		t.Fatalf("initialize node 1: %v", err)
	}

	node1.SetToken(adminToken)

	tok2, err := node1.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     2,
		TTLSeconds: 1800,
	})
	if err != nil {
		t.Fatalf("mint token for node 2: %v", err)
	}

	tok3, err := node1.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     3,
		TTLSeconds: 1800,
	})
	if err != nil {
		t.Fatalf("mint token for node 3: %v", err)
	}

	if err := writeNodeConfigOpts(composeDir, 2, fqdnPeers, tok2.Token, "", true); err != nil {
		t.Fatalf("write node 2 config: %v", err)
	}

	if err := writeNodeConfigOpts(composeDir, 3, fqdnPeers, tok3.Token, "", true); err != nil {
		t.Fatalf("write node 3 config: %v", err)
	}

	// --- Phase B: cold-restart node 1 alongside nodes 2 and 3. ---

	t.Log("phase B: stopping node 1 and starting all three concurrently")

	if err := dc.ComposeStopWithFile(ctx, composeDir, composeFile, "ella-core-1"); err != nil {
		t.Fatalf("stop node 1: %v", err)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile,
		"ella-core-1", "ella-core-2", "ella-core-3"); err != nil {
		t.Fatalf("start all nodes: %v", err)
	}

	// --- Phase C: assert convergence within a tight deadline. ---

	clients, err := newHANodeClients()
	if err != nil {
		t.Fatalf("build node clients: %v", err)
	}

	for _, c := range clients {
		c.SetToken(adminToken)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dc, composeDir, haNodeServices, clients)
	})

	if err := waitForClusterReadyWithin(ctx, clients, 60*time.Second); err != nil {
		t.Fatalf("cluster failed to converge after concurrent bootstrap: %v", err)
	}
}
