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

// TS 23.502 §4.9.1.3.3
func TestIntegration5GN2Handover(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if f := DetectIPFamily(); f == IPv6Only || f == DualStack {
		t.Skipf("skipping: TestIntegration5GN2Handover is IPv4-only (IP_VERSION=%s)", os.Getenv("IP_VERSION"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	const (
		composeDir  = "compose/n2-handover/"
		composeFile = "compose.yaml"
		coreAPI     = "http://10.3.0.2:5002"
		coreN2      = "10.3.0.2:38412"
	)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	// Clean up any lingering compose stacks.
	dc.ComposeCleanup(ctx)

	// Bring up the stack.
	if err := dc.ComposeUpWithFile(ctx, composeDir, composeFile); err != nil {
		t.Fatalf("compose up: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		for _, svc := range []string{"ella-core", "ella-core-tester"} {
			logs, logErr := dc.ComposeLogs(cleanupCtx, composeDir, svc)
			if logErr != nil {
				if t.Failed() {
					t.Logf("=== %s logs: collection failed: %v ===", svc, logErr)
				}
			} else if t.Failed() {
				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		dc.ComposeDownWithFile(cleanupCtx, composeDir, composeFile)
	})

	// Wait for core readiness.
	coreClient, err := client.New(&client.Config{
		BaseURLs: []string{coreAPI},
	})
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

	// Configure NAT and route via the Ella Core API.
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

	// Provision baseline resources.
	fx := fixture.New(t, ctx, coreClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	// Provision subscribers for both scenarios.
	scenarioSpecsByName := map[string]scenarios.FixtureSpec{}

	for _, name := range []string{"gnb/ngap/n2_handover", "gnb/n2_handover_connectivity"} {
		s, ok := scenarios.Get(name)
		if !ok || s.Fixture == nil {
			continue
		}

		spec := s.Fixture(scenarios.Env{})
		scenarioSpecsByName[name] = spec

		fx.Apply(spec)
	}

	// Resolve tester container.
	testerContainer, err := dc.ResolveComposeContainer(ctx, "n2-handover", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	// Run scenarios from the single tester container which has both
	// gNB addresses (source and target) in its network namespace.
	type scenarioRun struct {
		name string
	}

	scenariosToRun := []scenarioRun{
		{name: "gnb/ngap/n2_handover"},
		{name: "gnb/n2_handover_connectivity"},
	}

	for _, sr := range scenariosToRun {
		t.Run(sr.name, func(t *testing.T) {
			argv := []string{
				"core-tester", "run", sr.name,
				"--ella-core-n2-address", coreN2,
				"--gnb", "source,n2=10.3.0.3,n3=10.3.0.21",
				"--gnb", "target,n2=10.3.0.4,n3=10.3.0.22",
				"--verbose",
			}

			out, execErr := dc.Exec(ctx, testerContainer, argv, false, 3*time.Minute, nil)
			if execErr != nil {
				t.Fatalf("scenario %s failed: %v\n--- output ---\n%s", sr.name, execErr, out)
			}

			t.Logf("scenario %s passed\n%s", sr.name, out)

			if spec, ok := scenarioSpecsByName[sr.name]; ok && len(spec.AssertUsageForIMSIs) > 0 {
				fixture.AssertUsagePositive(ctx, t, coreClient, spec.AssertUsageForIMSIs, 30*time.Second)
			}
		})
	}
}
