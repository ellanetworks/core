// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

func runUE2UESuite(t *testing.T, rat string) {
	t.Helper()

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	if err := env.Client.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: false}); err != nil {
		t.Fatalf("disable NAT: %v", err)
	}

	origLS, err := env.Client.GetLocalSwitchInfo(ctx)
	if err != nil {
		t.Fatalf("get local switch (baseline): %v", err)
	}

	t.Cleanup(func() {
		_ = env.Client.UpdateLocalSwitchInfo(ctx, &client.UpdateLocalSwitchInfoOptions{Enabled: origLS.Enabled})
	})

	scenario := rat + "/ue2ue"

	sc, ok := scenarios.Get(scenario)
	if !ok {
		t.Fatalf("scenario %q not registered", scenario)
	}

	spec := sc.Fixture(scenarios.Env{})

	testCases := []struct {
		name          string
		localSwitch   bool
		expectSuccess bool
	}{
		{name: "local switch enabled", localSwitch: true, expectSuccess: true},
		{name: "local switch disabled", localSwitch: false, expectSuccess: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := env.Client.UpdateLocalSwitchInfo(ctx, &client.UpdateLocalSwitchInfoOptions{Enabled: tc.localSwitch}); err != nil {
				t.Fatalf("set local switch=%t: %v", tc.localSwitch, err)
			}

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(spec)

			tr := registerScenarioTest(scenario)
			defer finishScenarioTest(t, tr)

			var extraArgs []string
			if tc.expectSuccess {
				extraArgs = append(extraArgs, "--expect-success")
			} else {
				extraArgs = append(extraArgs, "--expect-success=false")
			}

			env.RunScenario(ctx, t, scenario, tr, extraArgs...)

			if globalReporter.FailureCount() > 0 {
				writeFailureReports(t, fmt.Sprintf("ue2ue-%s", rat))
			}
		})
	}
}
