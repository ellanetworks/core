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

// TestIntegration4GSessionModification drives a 4G UE through
// s1enb/session-modification* to assert the MME applies a mid-session policy edit
// in place (TS 24.301 §6.4.2): a QCI/ARP change arrives in an S1AP E-RAB Modify
// Request (TS 36.413 §8.2.2) with the new EPS QoS piggybacked in the NAS-PDU, and
// a Session-AMBR change arrives in a Modify EPS Bearer Context Request carrying the
// new APN-AMBR — without re-establishing the bearer.
func TestIntegration4GSessionModification(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skipf("TestIntegration4GSessionModification runs in IPv4 mode, current %s", DetectIPFamily())
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
		scenarios.DefaultSubscriberWith("001017271246651", ""),
	}})

	scenarioNames := []string{
		"s1enb/session-modification-ambr-only",
		"s1enb/session-modification-qos-only",
		"s1enb/session-modification",
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
