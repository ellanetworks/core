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
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration4GS1Handover runs the full inter-eNB S1 handover scenario with
// data-plane continuity against a single core with two eNB radios.
//
// Topology: 1 Ella Core + 1 tester container holding both eNB identities + 1
// router (the same two-eNB topology as the X2 handover test). The after-ping
// proves the MME completed the S1 handover (HANDOVER REQUIRED → REQUEST →
// ACKNOWLEDGE → COMMAND → STATUS TRANSFER → NOTIFY) and switched the UPF downlink
// to the target eNB only at notify (TS 36.413 §8.4, TS 23.401 §5.5.1.2.2).
func TestIntegration4GS1Handover(t *testing.T) {
	suites.RequireAll(t, suites.Handover4G, suites.Handover4GVRF)

	if DetectIPFamily() == DualStack {
		t.Skipf("skipping: TestIntegration4GS1Handover has no dualstack topology (IP_VERSION=%s)", os.Getenv("IP_VERSION"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	const composeDir = "compose/x2-handover/"

	scenariosToRun := []string{"s1enb/s1_handover", "s1enb/s1_handover_indirect_forwarding", "s1enb/s1_handover_ping_pong"}

	composeFiles := withVRFOverlay(ctx, t, HandoverComposeFile())
	coreAPI := APIAddress()
	coreN2 := HandoverCoreN2Address()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	dc.ComposeCleanup(ctx)

	if err := dc.ComposeUpWithFiles(ctx, composeDir, composeFiles...); err != nil {
		t.Fatalf("compose up: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		captureServiceLogs(t, dc, composeDir, []string{"ella-core", "ella-core-tester"})

		dc.ComposeDownWithFiles(cleanupCtx, composeDir, composeFiles...)
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

	fx := fixture.New(t, ctx, coreClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	specsByName := map[string]scenarios.FixtureSpec{}

	for _, name := range scenariosToRun {
		s, ok := scenarios.Get(name)
		if !ok || s.Fixture == nil {
			continue
		}

		spec := s.Fixture(scenarios.Env{})
		specsByName[name] = spec

		fx.Apply(spec)
	}

	testerContainer, err := dc.ResolveComposeContainer(ctx, "x2-handover", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	for _, scenario := range scenariosToRun {
		t.Run(scenario, func(t *testing.T) {
			argv := []string{
				"core-tester", "run", scenario,
				"--ella-core-n2-address", coreN2,
				"--ip-version", string(DetectIPFamily()),
				"--verbose",
			}

			for _, radio := range HandoverRadioSpecs() {
				argv = append(argv, "--gnb", radio)
			}

			out, execErr := dc.Exec(ctx, testerContainer, argv, false, 3*time.Minute, nil)
			if execErr != nil {
				t.Fatalf("scenario %s failed: %v\n--- output ---\n%s", scenario, execErr, out)
			}

			t.Logf("scenario %s passed\n%s", scenario, out)

			if spec, ok := specsByName[scenario]; ok && len(spec.AssertUsageForIMSIs) > 0 {
				fixture.AssertUsagePositive(ctx, t, coreClient, spec.AssertUsageForIMSIs, 30*time.Second)
			}
		})
	}
}
