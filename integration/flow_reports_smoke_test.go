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
// baseline, in both directions.
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

	env.RunScenario(ctx, t, fp.scenarioAllowed, spec.ExtraArgs...)

	scenarioEnd := time.Now()

	// UPF only exports a flow once it has been idle for ~30 s, so
	// visibility lags the last packet by up to ~45 s.
	for _, direction := range []string{"uplink", "downlink"} {
		flows := fixture.AssertFlowReports(
			ctx, t, env.Client,
			&client.ListFlowReportsParams{
				Direction:   direction,
				Action:      "allow",
				Protocol:    fp.protocolFilter,
				Source:      apiSourceIPFilter(direction, fp),
				Destination: apiDestinationIPFilter(direction, fp),
				PerPage:     100,
			},
			expectedFlowsContentPredicate(direction, "allow", expectedIMSIs, fp),
			90*time.Second,
		)

		fixture.AssertEachBytesIs(t, flows, expectedBytesPerFlow(fp))
		fixture.AssertEachTimestampsWithin(t, flows, scenarioStart, scenarioEnd.Add(timestampUpperBuffer))

		t.Logf("%s allow flows: %d", direction, len(flows))
	}
}
