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

// TestS1Reset4G drives a 4G eNB through s1enb/s1_reset: after a UE attaches, the
// eNB resets the whole S1 interface (TS 36.413 §8.7.1) and the MME must answer
// with a RESET ACKNOWLEDGE carrying no connection list, drop the UE's S1
// context, and keep the association up so a subsequent attach succeeds.
func TestS1Reset4G(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() == IPv6Only {
		t.Skipf("TestS1Reset4G requires an IPv4 PDN, current %s", DetectIPFamily())
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
		scenarios.DefaultSubscriberWith("001017271246614", ""),
	}})

	const name = "s1enb/s1_reset"

	tr := globalReporter.Start(name)
	QuietLogf(t, tr, "running %s", name)
	env.RunScenario(ctx, t, name, tr)
	finishScenarioTest(t, tr)
}
