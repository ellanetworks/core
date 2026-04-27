package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	// Side-effect import to register every scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// scenariosSkipped lists scenarios the integration suite does not
// exercise (multi-gNB and paging are out of scope).
var scenariosSkipped = map[string]string{
	"gnb/ngap/xn_handover":        "multi-gNB, out of scope",
	"ue/xn_handover_connectivity": "multi-gNB, out of scope",
	"ue/paging/downlink_data":     "paging, out of scope",
	"ha/failover_connectivity":    "multi-core HA topology, covered by TestIntegration3GPPHAFailover",
	"multi/cluster_traffic":       "multi-core HA topology, covered by TestIntegration3GPPMultiGNB",
}

// TestIntegrationTester brings the core-tester compose up once,
// bootstraps Ella Core with the baseline operator, default profile,
// slice, data network, and policy, then runs one subtest per registered
// scenario. Each subtest applies the scenario's FixtureSpec (with
// t.Cleanup teardown), invokes env.RunScenario, and polls the usage API
// when AssertUsageForIMSIs is set.
func TestIntegrationTester(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	t.Logf("core-tester compose up in %s mode", DetectIPFamily())

	// Baseline resources shared across all subtests.
	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	for _, name := range scenarios.List() {
		name := name

		if reason, skip := scenariosSkipped[name]; skip {
			t.Run(name, func(t *testing.T) { t.Skipf("%s: %s", name, reason) })
			continue
		}

		sc, _ := scenarios.Get(name)

		var spec scenarios.FixtureSpec
		if sc.Fixture != nil {
			spec = sc.Fixture()
		}

		t.Run(name, func(t *testing.T) {
			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(spec)

			env.RunScenario(ctx, t, name, spec.ExtraArgs...)

			if len(spec.AssertUsageForIMSIs) > 0 {
				fixture.AssertUsagePositive(ctx, t, env.Client, spec.AssertUsageForIMSIs, 30*time.Second)
			}
		})
	}
}
