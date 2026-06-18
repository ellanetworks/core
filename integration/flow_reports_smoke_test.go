// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestFlowReportsSmoke runs an icmp, udp, and tcp probe back-to-back
// against the empty-rules baseline, then asserts per-protocol flow
// content. Batching the probes amortises the UPF's per-flow flush
// latency across all three protocols.
func TestFlowReportsSmoke(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	fp := familyParams(DetectIPFamily())

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	sc, ok := scenarios.Get(fp.scenarioAllowed)
	Assert(t, ok, fmt.Sprintf("scenario %q not registered", fp.scenarioAllowed))

	scenariosEnv := buildScenariosEnv(env)

	var spec scenarios.FixtureSpec
	if sc.Fixture != nil {
		spec = sc.Fixture(scenariosEnv)
	}

	expectedIMSIs := make([]string, len(spec.Subscribers))
	for i, s := range spec.Subscribers {
		expectedIMSIs[i] = s.IMSI
	}

	fx := fixture.New(t, ctx, env.Client)
	fx.Apply(spec)

	if err := env.Client.ClearFlowReports(ctx); err != nil {
		t.Fatalf("clear flow reports: %v", err)
	}

	protocols := []string{"icmp", "udp", "tcp"}

	pps := make(map[string]probeProtocolParams, len(protocols))
	for _, p := range protocols {
		pps[p] = protocolParams(fp.family, p)
	}

	scenarioStart := time.Now()

	for _, p := range protocols {
		pp := pps[p]
		scenarioName := fmt.Sprintf("%s/%s", fp.scenarioAllowed, p)
		tr := globalReporter.Start(scenarioName)
		QuietLogf(t, tr, "running %s", scenarioName)
		env.RunScenario(ctx, t, fp.scenarioAllowed, tr, scenarioRunArgs(fp.scenarioAllowed, spec, pp)...)
		finishScenarioTest(t, tr)
	}

	scenarioEnd := time.Now()

	for _, p := range protocols {
		pp := pps[p]
		t.Run(p, func(t *testing.T) {
			for _, direction := range []string{"uplink", "downlink"} {
				flows := fixture.AssertFlowReports(
					ctx, t, env.Client,
					&client.ListFlowReportsParams{
						Direction:   direction,
						Action:      "allow",
						Protocol:    apiProtocolFilter(pp),
						Source:      apiSourceIPFilter(direction, fp),
						Destination: apiDestinationIPFilter(direction, fp),
						PerPage:     100,
					},
					expectedFlowsContentPredicate(direction, "allow", expectedIMSIs, fp, pp),
					90*time.Second,
				)

				if b := expectedBytesPerFlow(pp, direction); b != nil {
					fixture.AssertEachBytesIs(t, flows, *b)
				}

				if pp.packetsPerFlow == nil {
					for i, f := range flows {
						t.Logf("flow %d (imsi=%s dir=%s sp=%d dp=%d): packets=%d bytes=%d", i, f.SubscriberID, f.Direction, f.SourcePort, f.DestinationPort, f.Packets, f.Bytes)
					}
				}

				fixture.AssertEachTimestampsWithin(t, flows, scenarioStart, scenarioEnd.Add(timestampUpperBuffer))

				t.Logf("%s allow flows: %d", direction, len(flows))
			}
		})
	}
}
