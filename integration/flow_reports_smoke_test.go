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

// TestFlowReportsSmoke asserts per-flow content for the empty-rules
// baseline, in both directions, once per (icmp, tcp, udp).
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

	for _, protoName := range []string{"icmp", "udp", "tcp"} {
		pp := protocolParams(fp.family, protoName)

		t.Run(protoName, func(t *testing.T) {
			runSmoke(ctx, t, env, fp, pp)
		})
	}
}

func runSmoke(ctx context.Context, t *testing.T, env *testerEnv, fp ipFamilyParams, pp probeProtocolParams) {
	t.Helper()

	sc, ok := scenarios.Get(fp.scenarioAllowed)
	if !ok {
		t.Fatalf("scenario %q not registered", fp.scenarioAllowed)
	}

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

	scenarioStart := time.Now()

	env.RunScenario(ctx, t, fp.scenarioAllowed, scenarioRunArgs(fp.scenarioAllowed, spec, pp)...)

	scenarioEnd := time.Now()

	// UPF only exports a flow once it has been idle for ~30 s, so
	// visibility lags the last packet by up to ~45 s.
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
				t.Logf("flow %d (imsi=%s dir=%s): packets=%d bytes=%d", i, f.SubscriberID, f.Direction, f.Packets, f.Bytes)
			}
		}

		fixture.AssertEachTimestampsWithin(t, flows, scenarioStart, scenarioEnd.Add(timestampUpperBuffer))

		t.Logf("%s allow flows: %d", direction, len(flows))
	}
}
