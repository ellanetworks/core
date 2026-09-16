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
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

var vrfDatapathScenarios = []string{
	"gnb/connectivity",
	"gnb/connectivity_ipv6",
	"gnb/buffered_downlink",
}

var vrfLocalSwitchScenarios = map[string]bool{
	"gnb/buffered_downlink": true,
}

func TestIntegrationTesterVRF(t *testing.T) {
	suites.RequireAll(t, suites.Datapath5GVRF)

	if os.Getenv("VRF") != "" && DetectIPFamily() != IPv4Only {
		t.Setenv("VRF_UP_ROUTES", "")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	VerboseLogf(t, "core-tester compose up in %s mode with VRF overlay", string(DetectIPFamily()))

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	for _, name := range vrfDatapathScenarios {
		name := name

		if requiredFamily, ok := scenarioIPFamilyRestrictions[name]; ok && DetectIPFamily() != requiredFamily {
			tr := registerScenarioTest(name)
			t.Run(name, func(t *testing.T) {
				t.Skipf("skipping %s: requires %s mode", name, requiredFamily)
			})
			skipScenarioTest(tr, "unsupported IP family")

			continue
		}

		if exclusions, ok := scenarioIPFamilyExclusions[name]; ok && exclusions[DetectIPFamily()] {
			tr := registerScenarioTest(name)
			t.Run(name, func(t *testing.T) {
				t.Skipf("skipping %s in %s mode", name, DetectIPFamily())
			})
			skipScenarioTest(tr, "unsupported IP family")

			continue
		}

		sc, ok := scenarios.Get(name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", name))

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

			if vrfLocalSwitchScenarios[name] {
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
			}

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(spec)

			var extraArgs []string

			if requiredFamily, ok := scenarioIPFamilyRestrictions[name]; ok {
				extraArgs = append(extraArgs, "--ip-version", string(requiredFamily))
			}

			extraArgs = append(extraArgs, spec.ExtraArgs...)

			env.RunScenario(ctx, t, name, tr, extraArgs...)

			if len(spec.AssertUsageForIMSIs) > 0 {
				fixture.AssertUsagePositive(ctx, t, env.Client, spec.AssertUsageForIMSIs, 30*time.Second)
			}
		})
	}

	printTesterSummary(t)
}
