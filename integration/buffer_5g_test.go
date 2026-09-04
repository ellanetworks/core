// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration5GBufferedDownlink runs the gnb buffered-downlink scenario.
// TS 23.501 §5.8.2.2.1
func TestIntegration5GBufferedDownlink(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runBufferedSuite(t, "gnb/buffered_downlink")
}

func runBufferedSuite(t *testing.T, scenario string) {
	t.Helper()

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	origLS, err := env.Client.GetLocalSwitchInfo(ctx)
	if err != nil {
		t.Fatalf("get local switch (baseline): %v", err)
	}

	t.Cleanup(func() {
		_ = env.Client.UpdateLocalSwitchInfo(ctx, &client.UpdateLocalSwitchInfoOptions{Enabled: origLS.Enabled})
	})

	if err := env.Client.UpdateLocalSwitchInfo(ctx, &client.UpdateLocalSwitchInfoOptions{Enabled: true}); err != nil {
		t.Fatalf("enable local switch: %v", err)
	}

	sc, ok := scenarios.Get(scenario)
	if !ok {
		t.Fatalf("scenario %q not registered", scenario)
	}

	fx := fixture.New(t, ctx, env.Client)
	fx.Apply(sc.Fixture(scenarios.Env{}))

	tr := registerScenarioTest(scenario)

	t.Run(scenario, func(t *testing.T) {
		defer finishScenarioTest(t, tr)

		env.RunScenario(ctx, t, scenario, tr)
	})

	if globalReporter.FailureCount() > 0 {
		writeFailureReports(t, "buffered")
	}
}
