// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration5GXnHandover runs the Xn handover (target-gNB PATH SWITCH)
// scenario with data-plane continuity against a single core with two gNB tester
// containers.
//
// Topology: 1 Ella Core + 2 gNB testers (source + target) + 1 router.
// Compose: integration/compose/n2-handover/compose.yaml
//
// Per 3GPP TS 23.502 §4.9.1.2, the target NG-RAN node switches the downlink
// path itself once the UE has arrived; the after-ping proves the AMF's path
// switch handler reprogrammed the UPF downlink to the target gNB.
func TestIntegration5GXnHandover(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if f := DetectIPFamily(); f == IPv6Only || f == DualStack {
		t.Skipf("skipping: TestIntegration5GXnHandover is IPv4-only (IP_VERSION=%s)", os.Getenv("IP_VERSION"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	const (
		composeDir  = "compose/n2-handover/"
		composeFile = "compose.yaml"
		coreAPI     = "http://10.3.0.2:5002"
		coreN2      = "10.3.0.2:38412"
		scenario    = "gnb/xn_handover_connectivity"
	)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	dc.ComposeCleanup(ctx)

	if err := dc.ComposeUpWithFile(ctx, composeDir, composeFile); err != nil {
		t.Fatalf("compose up: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		for _, svc := range []string{"ella-core", "ella-core-tester"} {
			if logs, logErr := dc.ComposeLogs(cleanupCtx, composeDir, svc); logErr == nil && t.Failed() {
				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		dc.ComposeDownWithFile(cleanupCtx, composeDir, composeFile)
	})

	coreClient, err := client.New(&client.Config{BaseURLs: []string{coreAPI}})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	if err := waitForNodeReady(ctx, coreClient); err != nil {
		t.Fatalf("wait for core ready: %v", err)
	}

	adminToken, err := initializeAndGetAdminToken(ctx, coreClient)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	coreClient.SetToken(adminToken)

	if err := coreClient.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: true}); err != nil {
		t.Fatalf("enable NAT: %v", err)
	}

	if err := coreClient.CreateRoute(ctx, &client.CreateRouteOptions{
		Destination: "8.8.8.8/32",
		Gateway:     "10.6.0.3",
		Interface:   "n6",
		Metric:      0,
	}); err != nil {
		t.Fatalf("create route: %v", err)
	}

	fx := fixture.New(t, ctx, coreClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	spec := scenarios.FixtureSpec{}

	if s, ok := scenarios.Get(scenario); ok && s.Fixture != nil {
		spec = s.Fixture(scenarios.Env{})
		fx.Apply(spec)
	}

	testerContainer, err := dc.ResolveComposeContainer(ctx, "n2-handover", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	argv := []string{
		"core-tester", "run", scenario,
		"--ella-core-n2-address", coreN2,
		"--gnb", "source,n2=10.3.0.3,n3=10.3.0.21",
		"--gnb", "target,n2=10.3.0.4,n3=10.3.0.22",
		"--verbose",
	}

	out, execErr := dc.Exec(ctx, testerContainer, argv, false, 3*time.Minute, nil)
	if execErr != nil {
		t.Fatalf("scenario %s failed: %v\n--- output ---\n%s", scenario, execErr, out)
	}

	t.Logf("scenario %s passed\n%s", scenario, out)

	if len(spec.AssertUsageForIMSIs) > 0 {
		fixture.AssertUsagePositive(ctx, t, coreClient, spec.AssertUsageForIMSIs, 30*time.Second)
	}
}
