// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	// Side-effect import to register every scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// scenariosSkipped lists scenarios the integration suite does not exercise,
// mapped to why.
var scenariosSkipped = map[string]string{
	"gnb/ngap/n2_handover":                   "multi-gNB, covered by TestIntegration5GN2Handover",
	"gnb/n2_handover_connectivity":           "multi-gNB, covered by TestIntegration5GN2Handover",
	"gnb/xn_handover_connectivity":           "multi-gNB datapath, covered by TestIntegration5GXnHandover",
	"s1enb/x2_handover_connectivity":         "multi-eNB datapath, covered by TestIntegration4GX2Handover",
	"s1enb/s1_handover":                      "multi-eNB datapath, covered by TestIntegration4GS1Handover",
	"ha/failover_connectivity_5g":            "multi-core HA topology, covered by TestIntegration5GHAFailover",
	"ha/failover_connectivity_4g":            "multi-core HA topology, covered by TestIntegration4GHAFailover",
	"ha/drain_4g":                            "multi-core HA topology, covered by TestIntegration4GHADrain",
	"ha/drain_5g":                            "multi-core HA topology, covered by TestIntegration5GHADrain",
	"multi/cluster_traffic_5g":               "multi-core HA topology, covered by TestIntegration5GMultiGNB",
	"gnb/connectivity_expect_blocked":        "test-only harness; requires a pre-installed deny rule",
	"gnb/connectivity_expect_allowed":        "test-only harness; minimal allow-path",
	"gnb/connectivity_expect_blocked_ipv6":   "test-only harness; requires a pre-installed deny rule",
	"gnb/connectivity_expect_allowed_ipv6":   "test-only harness; minimal allow-path",
	"gnb/session_hold":                       "long-lived session for BGP tests; covered by TestIntegration5GBGP",
	"s1enb/session_hold":                     "long-lived session for BGP tests; covered by TestIntegration4GBGP",
	"gnb/nat_checksum":                       "capture-driven harness; covered by TestIntegration5GUPFNATChecksum",
	"s1enb/nat_checksum":                     "capture-driven harness; covered by TestIntegration4GUPFNATChecksum",
	"s1enb/connectivity_expect_allowed":      "test-only harness; driven by TestIntegration4GNetworkRules",
	"s1enb/connectivity_expect_blocked":      "test-only harness; driven by TestIntegration4GNetworkRules",
	"s1enb/connectivity_expect_allowed_ipv6": "test-only harness; driven by TestIntegration4GNetworkRules",
	"s1enb/connectivity_expect_blocked_ipv6": "test-only harness; driven by TestIntegration4GNetworkRules",
	"gnb/framed_route":                       "requires NAT disabled; covered by TestIntegration5GFramedRouting",
	"gnb/framed_route_ipv6":                  "requires NAT disabled; covered by TestIntegration5GFramedRouting",
	"gnb/framed_route_add_live":              "requires NAT disabled; covered by TestIntegration5GFramedRoutingReconcile",
	"gnb/framed_route_remove_live":           "requires NAT disabled; covered by TestIntegration5GFramedRoutingReconcile",
	"s1enb/framed_route":                     "requires NAT disabled; covered by TestIntegration4GFramedRouting",
	"s1enb/framed_route_ipv6":                "requires NAT disabled; covered by TestIntegration4GFramedRouting",
	"gnb/ue2ue":                              "requires NAT disabled; covered by TestIntegration5GUE2UE",
	"s1enb/ue2ue":                            "requires NAT disabled; covered by TestIntegration4GUE2UE",
}

// scenarioIPFamilyRestrictions returns a map of scenario name → required IP
// family. Scenarios that only make sense in a specific address-family
// configuration are listed here so the integration runner can skip them
// when the compose topology does not match.
var scenarioIPFamilyRestrictions = map[string]IPFamily{
	"gnb/connectivity_ipv6":                  IPv6Only,
	"gnb/connectivity_dualstack":             DualStack,
	"gnb/connectivity_expect_allowed_ipv6":   IPv6Only,
	"gnb/connectivity_expect_blocked_ipv6":   IPv6Only,
	"s1enb/connectivity_expect_allowed_ipv6": IPv6Only,
	"s1enb/connectivity_expect_blocked_ipv6": IPv6Only,
	"s1enb/registration/v4v6":                DualStack,
	"s1enb/connectivity_dualstack":           DualStack,
	"s1enb/connectivity_ipv6":                IPv6Only,
	"gnb/static_ip_ipv6":                     IPv6Only,
	"s1enb/static_ip_ipv6":                   IPv6Only,
}

var scenarioFollowsDeploymentIPFamily = map[string]bool{
	"interworking/transfer_5gs_to_eps":                true,
	"interworking/transfer_eps_to_5gs":                true,
	"interworking/handover_5gs_to_eps":                true,
	"interworking/handover_5gs_to_eps_target_refuses": true,
	"interworking/handover_eps_to_5gs":                true,
	"interworking/handover_eps_to_5gs_target_refuses": true,
	"interworking/idle_5gs_to_eps":                    true,
	"interworking/idle_5gs_to_eps_returning_to_idle":  true,
	"interworking/idle_eps_to_5gs":                    true,
	"interworking/idle_eps_to_5gs_returning_to_idle":  true,
	"interworking/idle_round_trip_through_eps":        true,
	"interworking/idle_round_trip_through_5gs":        true,
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
	"s1enb/connectivity": {
		IPv6Only:  true,
		DualStack: true,
	},
	"gnb/static_ip": {
		IPv6Only:  true,
		DualStack: true,
	},
	"gnb/framed_route": {
		IPv6Only:  true,
		DualStack: true,
	},
	"s1enb/static_ip": {
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
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

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

	// Track which scenarios to run (filtering skipped/restricted ones).
	var scenarioNames []string

	scenarioNames = append(scenarioNames, scenarios.List()...)

	// Run each scenario with reporter tracking.
	for _, name := range scenarioNames {
		name := name

		if reason, skip := scenariosSkipped[name]; skip {
			tr := registerScenarioTest(name)
			t.Run(name, func(t *testing.T) { t.Skipf("%s: %s", name, reason) })
			skipScenarioTest(tr, reason)

			continue
		}

		if requiredFamily, ok := scenarioIPFamilyRestrictions[name]; ok {
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

			if _, both := scenarioIPFamilyRestrictions[name]; both && scenarioFollowsDeploymentIPFamily[name] {
				t.Fatalf("scenario %q is both pinned to one IP family and told to follow the deployment's", name)
			}

			switch {
			case scenarioFollowsDeploymentIPFamily[name]:
				extraArgs = append(extraArgs, "--ip-version", string(DetectIPFamily()))
			default:
				if requiredFamily, ok := scenarioIPFamilyRestrictions[name]; ok {
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
