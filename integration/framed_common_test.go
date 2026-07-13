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
	// Side-effect import to register every scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// runFramedSuite brings up the core-tester compose, disables NAT (framed routes
// have no function under NAT, TS 23.501 §5.6.14), and runs the framed-route
// scenario for the given RAN type ("gnb" for 5G, "s1enb" for 4G) in the address
// family selected by the compose topology. Framed routing is exercised in a
// dedicated suite because NAT is a global setting incompatible with the
// NAT-enabled scenarios of TestIntegrationTester.
func runFramedSuite(t *testing.T, rat string) {
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

	scenario := rat + "/framed_route"

	var extraArgs []string

	if DetectIPFamily() == IPv6Only {
		scenario += "_ipv6"

		extraArgs = append(extraArgs, "--ip-version", "ipv6")
	}

	sc, ok := scenarios.Get(scenario)
	if !ok {
		t.Fatalf("scenario %q not registered", scenario)
	}

	spec := sc.Fixture(scenarios.Env{})

	fx := fixture.New(t, ctx, env.Client)
	fx.Apply(spec)

	tr := registerScenarioTest(scenario)

	t.Run(scenario, func(t *testing.T) {
		defer finishScenarioTest(t, tr)

		env.RunScenario(ctx, t, scenario, tr, extraArgs...)
	})

	if globalReporter.FailureCount() > 0 {
		writeFailureReports(t, fmt.Sprintf("framed-%s", rat))
	}
}

// runFramedReconcileSuite brings up the core-tester compose with NAT disabled and
// runs the framed-route reconcile scenarios: adding and removing a framed route
// on a live session must release it with cause #39 for re-establishment
// (TS 23.501 §5.6.14).
func runFramedReconcileSuite(t *testing.T, scenarioNames ...string) {
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

	for _, scenario := range scenarioNames {
		sc, ok := scenarios.Get(scenario)
		if !ok {
			t.Fatalf("scenario %q not registered", scenario)
		}

		spec := sc.Fixture(scenarios.Env{})

		fx := fixture.New(t, ctx, env.Client)
		fx.Apply(spec)

		tr := registerScenarioTest(scenario)

		t.Run(scenario, func(t *testing.T) {
			defer finishScenarioTest(t, tr)

			env.RunScenario(ctx, t, scenario, tr)
		})
	}

	if globalReporter.FailureCount() > 0 {
		writeFailureReports(t, "framed-reconcile")
	}
}
