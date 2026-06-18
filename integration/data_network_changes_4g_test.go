// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestDataNetworkChanges4G drives a 4G UE through s1enb/data-network-*-change to
// assert the MME propagates a data-network reconfiguration to an active EPS
// bearer: a DNS, MTU, or IP-pool change deactivates the bearer with ESM cause
// #39 "reactivation requested" (TS 24.301 §6.4.4.2) and the UE re-attaches with
// the new configuration. This is the 4G counterpart of the AMF session
// reconciler exercised by ue/data-network-*-change.
func TestDataNetworkChanges4G(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skipf("TestDataNetworkChanges4G runs in IPv4 mode, current %s", DetectIPFamily())
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())
	baseline.Apply(scenarios.FixtureSpec{Subscribers: []scenarios.SubscriberSpec{
		scenarios.DefaultSubscriberWith("001017271246650", ""),
	}})

	scenarioNames := []string{
		"s1enb/data-network-dns-change",
		"s1enb/data-network-mtu-change",
		"s1enb/data-network-pool-change",
	}

	for _, name := range scenarioNames {
		name := name

		t.Run(name, func(t *testing.T) {
			tr := globalReporter.Start(name)
			QuietLogf(t, tr, "running %s", name)
			env.RunScenario(ctx, t, name, tr)
			finishScenarioTest(t, tr)
		})
	}
}
