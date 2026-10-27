// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const (
	imsSMSScenarioPrefix = "ims-sms/"
	ipSMGWNumber         = "15550000100"
)

func TestIntegrationIMSSMS(t *testing.T) {
	suites.Require(t, suites.IMSSMS)

	ctx := context.Background()
	env := setupTesterEnv(ctx, t, imsOverlay(), smscOverlay())

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	configureSMSC(ctx, t)
	enableSMS(ctx, t, env.Client, smscAddress())
	addIMSCorePeer(ctx, t, env.Client, "sh", "s6c", "sgd")
	configureIPSMGW(ctx, t, env.Client)

	for _, name := range scenarios.List() {
		if !strings.HasPrefix(name, imsSMSScenarioPrefix) {
			continue
		}

		sc, ok := scenarios.Get(name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", name))

		tr := registerScenarioTest(name)

		t.Run(name, func(t *testing.T) {
			defer finishScenarioTest(t, tr)

			if err := waitForCxLink(ctx, env.Client); err != nil {
				t.Fatal(err)
			}

			if err := waitForSMSCLink(ctx, env.Client); err != nil {
				t.Fatal(err)
			}

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(sc.Fixture(scenarios.Env{}))

			env.RunScenario(ctx, t, name, tr, "--ip-version", string(DetectIPFamily()), "--service-centre", smscServiceCentre)
		})
	}

	printTesterSummary(t)
}

func configureIPSMGW(ctx context.Context, t *testing.T, cl *client.Client) {
	t.Helper()

	status, err := cl.GetDiameterStatus(ctx)
	if err != nil {
		t.Fatalf("get the Diameter status: %v", err)
	}

	smsc := map[string]any{
		"host":            smscHost,
		"address":         smscAddress(),
		"port":            smscDiameterPort,
		"transport":       "sctp",
		"applications":    []string{"sgd"},
		"service_centres": []string{strings.TrimPrefix(smscServiceCentre, "+")},
	}

	steps := []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, "/api/v1/policy", map[string]string{"interface": "rx"}},
		{http.MethodPut, "/api/v1/ip-sm-gw", map[string]string{"number": ipSMGWNumber}},
		{http.MethodPut, "/api/v1/diameter/routes/sh", map[string]string{"realm": status.Realm}},
		{http.MethodPut, "/api/v1/diameter/routes/s6c", map[string]string{"realm": status.Realm}},
		{http.MethodPost, "/api/v1/diameter/peers", smsc},
	}

	for _, s := range steps {
		deadline := time.Now().Add(time.Minute)

		for {
			err := sendIMS(ctx, s.method, s.path, s.body)
			if err == nil {
				break
			}

			if time.Now().After(deadline) {
				t.Fatalf("configure the IP-SM-GW of Ella IMS: %v", err)
			}

			time.Sleep(time.Second)
		}
	}
}
