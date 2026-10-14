// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	// Side-effect import to register every scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

func scenarioRequiredFamily(name string) (IPFamily, bool) {
	s, ok := scenarios.Get(name)
	if !ok || s.OnlyFamily == nil {
		return "", false
	}

	switch *s.OnlyFamily {
	case scenarios.IPv6Only:
		return IPv6Only, true
	case scenarios.DualStack:
		return DualStack, true
	default:
		return IPv4Only, true
	}
}

var scenarioFollowsDeploymentIPFamily = map[string]bool{
	"interworking/transfer_5gs_to_eps":                  true,
	"interworking/transfer_eps_to_5gs":                  true,
	"interworking/handover_5gs_to_eps":                  true,
	"interworking/handover_5gs_to_eps_target_refuses":   true,
	"interworking/handover_eps_to_5gs":                  true,
	"interworking/handover_eps_to_5gs_target_refuses":   true,
	"interworking/idle_5gs_to_eps":                      true,
	"interworking/idle_5gs_to_eps_returning_to_idle":    true,
	"interworking/idle_eps_to_5gs":                      true,
	"interworking/idle_eps_to_5gs_returning_to_idle":    true,
	"interworking/idle_round_trip_through_eps":          true,
	"interworking/idle_round_trip_through_5gs":          true,
	"interworking/idle_eps_to_5gs_bearer_status":        true,
	"interworking/idle_eps_to_5gs_session_modification": true,
}

// scenarioIPFamilyExclusions returns a map of scenario name → set of IP
// families in which the scenario should be skipped. This is used for
// scenarios that test a specific address family but should be skipped
// when N6 does not have that family configured.
var scenarioIPFamilyExclusions = map[string]map[IPFamily]bool{
	"gnb/connectivity": {
		IPv6Only: true,
	},
	"gnb/connectivity_ipv6": {
		IPv4Only: true,
	},
	"gnb/connectivity_multi_pdu_session": {
		IPv6Only: true,
	},
	"gnb/service_request_pdu_session_status": {
		IPv6Only: true,
	},
	"gnb/mobility_registration_pdu_session_status": {
		IPv6Only: true,
	},
	"gnb/connectivity_multiple_policies_per_profile": {
		IPv6Only: true,
	},
	"enb/connectivity": {
		IPv6Only:  true,
		DualStack: true,
	},
	"s1enb/connectivity_multi_pdn": {
		IPv6Only: true,
	},
	"s1enb/tau_bearer_context_status": {
		IPv6Only: true,
	},
	"s1enb/connectivity": {
		IPv6Only:  true,
		DualStack: true,
	},
}

// TestIntegrationTester brings the core-tester compose up once,
// bootstraps Ella Core with the baseline operator, default profile,
// slice, data network, and policy, then runs one subtest per registered
// scenario. Each subtest applies the scenario's FixtureSpec (with
// t.Cleanup teardown), invokes env.RunScenario, and polls the usage API
// when AssertUsageForIMSIs is set.
func TestIntegrationTester(t *testing.T) {
	suites.RequireSplit(t, "s1enb", suites.Datapath4G, suites.Datapath5G)

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	VerboseLogf(t, "core-tester compose up in %s mode", string(DetectIPFamily()))

	// Baseline resources shared across all subtests.
	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	for _, name := range scenarios.ListTier(scenarios.TierCompose) {
		name := name

		if requiredFamily, ok := scenarioRequiredFamily(name); ok {
			if DetectIPFamily() != requiredFamily {
				reason := fmt.Sprintf("requires %s mode, running %s", requiredFamily, DetectIPFamily())

				tr := registerScenarioTest(name)
				t.Run(name, func(t *testing.T) {
					t.Skipf("skipping %s: %s", name, reason)
				})
				skipScenarioTest(tr, reason)

				continue
			}
		}

		if exclusions, ok := scenarioIPFamilyExclusions[name]; ok {
			if exclusions[DetectIPFamily()] {
				reason := fmt.Sprintf("N6 does not support this address family in %s mode", DetectIPFamily())

				tr := registerScenarioTest(name)
				t.Run(name, func(t *testing.T) {
					t.Skipf("skipping %s: %s", name, reason)
				})
				skipScenarioTest(tr, reason)

				continue
			}
		}

		sc, ok := scenarios.Get(name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", name))

		// Build a scenarios.Env from the tester environment so that
		// IP-family-aware fixtures can inspect address family details.
		var scenariosEnv scenarios.Env
		if len(env.CoreN2Addresses) > 0 || len(env.GNBs) > 0 {
			scenariosEnv = buildScenariosEnv(env)
		}

		var spec scenarios.FixtureSpec
		if sc.Fixture != nil {
			spec = sc.Fixture(scenariosEnv)
		}

		tr := registerScenarioTest(name)

		t.Run(name, func(t *testing.T) {
			defer finishScenarioTest(t, tr)

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(spec)

			var extraArgs []string

			if _, both := scenarioRequiredFamily(name); both && scenarioFollowsDeploymentIPFamily[name] {
				t.Fatalf("scenario %q is both pinned to one IP family and told to follow the deployment's", name)
			}

			switch {
			case scenarioFollowsDeploymentIPFamily[name]:
				extraArgs = append(extraArgs, "--ip-version", string(DetectIPFamily()))
			default:
				if requiredFamily, ok := scenarioRequiredFamily(name); ok {
					extraArgs = append(extraArgs, "--ip-version", string(requiredFamily))
				}
			}

			extraArgs = append(extraArgs, spec.ExtraArgs...)

			env.RunScenario(ctx, t, name, tr, extraArgs...)

			if len(spec.AssertUsageForIMSIs) > 0 {
				fixture.AssertUsagePositive(ctx, t, env.Client, spec.AssertUsageForIMSIs, 30*time.Second)
			}
		})
	}

	// Print summary at the end.
	printTesterSummary(t)
}

// buildScenariosEnv converts a *testerEnv into a scenarios.Env so that
// IP-family-aware fixtures can inspect the address family of the gNB N3
// interface.
func buildScenariosEnv(e *testerEnv) scenarios.Env {
	gnbs := make([]scenarios.GNB, 0, len(e.GNBs))
	for _, g := range e.GNBs {
		gnbs = append(gnbs, scenarios.GNB{
			Name:        g.Name,
			N2Address:   g.N2Address,
			N3Address:   g.N3Address,
			N3Secondary: g.N3Secondary,
		})
	}

	return scenarios.Env{
		CoreN2Addresses: e.CoreN2Addresses,
		GNBs:            gnbs,
	}
}
