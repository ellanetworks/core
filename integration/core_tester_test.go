package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	// Side-effect import to register every scenario via its init()
	// function.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// scenariosSkipped lists scenario names the current integration suite
// does not exercise. These are NOT regressions — they are explicitly out
// of scope per spec_tester_v2.md (multi-gNB and paging are deferred to
// later tiers).
var scenariosSkipped = map[string]string{
	"gnb/ngap/xn_handover":        "multi-gNB, out of scope",
	"ue/xn_handover_connectivity": "multi-gNB, out of scope",
	"ue/paging/downlink_data":     "paging, out of scope",
}

// TestIntegrationTester brings the core-tester compose up once, bootstraps
// Ella Core with the baseline operator + default profile/slice/DN/policy,
// then runs one subtest per registered scenario. The scenario's Fixture()
// declares any per-scenario resources; the subtest applies that fixture,
// invokes env.RunScenario, and (if declared) polls the usage API
// post-run.
func TestIntegrationTester(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	t.Logf("core-tester compose up in %s mode", DetectIPFamily())

	// Baseline: operator, default profile, default slice, default data
	// network, default policy. All scenarios share these via
	// scenarios.Default* constants. Idempotent — the fixture helpers
	// verify match if a resource already exists.
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
