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

// TestMultiPDN4G drives a 4G UE through s1enb/connectivity_multi_pdn: it attaches
// on the default APN, opens a second PDN connection to another APN, verifies
// user-plane connectivity on both with distinct UE IPs, then disconnects the
// second PDN. This is the 4G counterpart of ue/connectivity_multi_pdu_session.
func TestMultiPDN4G(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() == IPv6Only {
		t.Skipf("TestMultiPDN4G requires IPv4 PDNs, current %s", DetectIPFamily())
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	const name = "s1enb/connectivity_multi_pdn"

	sc, ok := scenarios.Get(name)
	if !ok {
		t.Fatalf("scenario %q not registered", name)
	}

	baseline.Apply(sc.Fixture(scenarios.Env{}))

	tr := globalReporter.Start(name)
	QuietLogf(t, tr, "running %s", name)
	env.RunScenario(ctx, t, name, tr)
	finishScenarioTest(t, tr)
}
