// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
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
	smsScenarioPrefix = "sms/"
	smscDiameterPort  = 3868
	smscAPIPort       = 5010
	smsNumber         = "+15550000000"
)

func smscAddress() string {
	if DetectIPFamily() == IPv4Only {
		return "10.3.0.5"
	}

	return "2001:db8:1::15"
}

func smscAPIAddress() string {
	return "http://" + net.JoinHostPort(smscAddress(), strconv.Itoa(smscAPIPort))
}

func smscOverlay() string {
	if DetectIPFamily() == IPv4Only {
		return "../sms/smsc.yaml"
	}

	return "../sms/smsc-ipv6.yaml"
}

func TestIntegrationSMS(t *testing.T) {
	suites.Require(t, suites.SMS)

	ctx := context.Background()
	env := setupTesterEnv(ctx, t, smscOverlay())

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	if err := env.Client.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{
		Enabled:     true,
		SMSCAddress: smscAddress(),
		SMSCPort:    smscDiameterPort,
		SMSNumber:   smsNumber,
	}); err != nil {
		t.Fatalf("enable SMS: %v", err)
	}

	for _, name := range scenarios.List() {
		if !strings.HasPrefix(name, smsScenarioPrefix) {
			continue
		}

		sc, ok := scenarios.Get(name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", name))

		tr := registerScenarioTest(name)

		t.Run(name, func(t *testing.T) {
			defer finishScenarioTest(t, tr)

			if err := waitForSMSCLink(ctx, env.Client); err != nil {
				t.Fatal(err)
			}

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(sc.Fixture(scenarios.Env{}))

			env.RunScenario(ctx, t, name, tr, "--smsc-api-address", smscAPIAddress())
		})
	}

	printTesterSummary(t)
}

func waitForSMSCLink(ctx context.Context, cl *client.Client) error {
	deadline := time.Now().Add(2 * time.Minute)

	for {
		coreOpen := false

		status, err := cl.GetDiameterStatus(ctx)
		if err == nil {
			for _, p := range status.Peers {
				coreOpen = coreOpen || (p.Role == "smsc" && p.State == "open")
			}
		}

		hss := smscSeesHSS(ctx)
		if coreOpen && hss {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("the Diameter link between Ella Core and the SMSC did not open (core status %+v, error %v, SMSC sees the HSS %t)", status, err, hss)
		}

		time.Sleep(time.Second)
	}
}

func smscSeesHSS(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, smscAPIAddress()+"/api/v1/diameter", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}

	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Result struct {
			HSSAvailable bool `json:"hss_available"`
		} `json:"result"`
	}

	return json.NewDecoder(resp.Body).Decode(&body) == nil && body.Result.HSSAvailable
}
