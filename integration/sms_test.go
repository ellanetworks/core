// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	smscServiceCentre = "+15550000000"
	smscHost          = "smsc.node.epc.mnc001.mcc001.3gppnetwork.org"
	smscAdminEmail    = "admin@ellanetworks.com"
	smscAdminPassword = "integration-test-password"
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

	smscToken := configureSMSC(ctx, t)
	enableSMS(ctx, t, env.Client, smscAddress())

	for _, name := range scenarios.List() {
		if !strings.HasPrefix(name, smsScenarioPrefix) {
			continue
		}

		sc, ok := scenarios.Get(name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", name))

		tr := registerScenarioTest(name)

		t.Run(name, func(t *testing.T) {
			defer finishScenarioTest(t, tr)

			if err := waitForSMSCLink(ctx, env.Client, smscToken); err != nil {
				t.Fatal(err)
			}

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(sc.Fixture(scenarios.Env{}))

			env.RunScenario(ctx, t, name, tr, "--smsc-api-address", smscAPIAddress(), "--smsc-api-token", smscToken)
		})
	}

	printTesterSummary(t)
}

func enableSMS(ctx context.Context, t *testing.T, cl *client.Client, address string) {
	t.Helper()

	if _, err := cl.CreateSMSCPeer(ctx, &client.SMSCPeerOptions{
		DiameterIdentity: smscHost,
		Address:          address,
		Port:             smscDiameterPort,
		ServiceCentres:   []string{smscServiceCentre},
	}); err != nil {
		t.Fatalf("add the SMSC peer: %v", err)
	}

	if err := cl.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{SMSNumber: smsNumber}); err != nil {
		t.Fatalf("enable SMS: %v", err)
	}
}

func waitForSMSCLink(ctx context.Context, cl *client.Client, smscToken string) error {
	deadline := time.Now().Add(2 * time.Minute)

	for {
		coreOpen := false

		status, err := cl.GetDiameterStatus(ctx)
		if err == nil {
			for _, p := range status.Peers {
				coreOpen = coreOpen || (p.Role == "smsc" && p.State == "open")
			}
		}

		hss := smscSeesHSS(ctx, smscToken)
		if coreOpen && hss {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("the Diameter link between Ella Core and the SMSC did not open (core status %+v, error %v, SMSC sees the HSS %t)", status, err, hss)
		}

		time.Sleep(time.Second)
	}
}

func configureSMSC(ctx context.Context, t *testing.T) string {
	t.Helper()

	operator := map[string]any{
		"mcc":                    "001",
		"mnc":                    "01",
		"service_centre_address": smscServiceCentre,
		"numbering":              map[string]string{"country_code": "1", "national_prefix": "1", "international_prefix": "011"},
	}

	delivery := map[string]any{
		"default_validity_seconds": 3600,
		"retry_intervals_seconds":  []int{600},
	}

	deadline := time.Now().Add(time.Minute)

	for {
		token, err := smscAPIToken(ctx)
		if err == nil {
			err = callSMSC(ctx, http.MethodPut, token, "/api/v1/operator", operator, nil)
		}

		if err == nil {
			err = callSMSC(ctx, http.MethodPut, token, "/api/v1/delivery", delivery, nil)
		}

		if err == nil {
			return token
		}

		if time.Now().After(deadline) {
			t.Fatalf("configure the SMSC: %v", err)
		}

		time.Sleep(time.Second)
	}
}

func smscAPIToken(ctx context.Context) (string, error) {
	credentials := map[string]string{"email": smscAdminEmail, "password": smscAdminPassword}

	var initialized struct {
		Token string `json:"token"`
	}

	if err := callSMSC(ctx, http.MethodPost, "", "/api/v1/init", credentials, &initialized); err != nil {
		if err := callSMSC(ctx, http.MethodPost, "", "/api/v1/auth/login", credentials, &initialized); err != nil {
			return "", err
		}
	}

	var created struct {
		Token string `json:"token"`
	}

	name := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	if err := callSMSC(ctx, http.MethodPost, initialized.Token, "/api/v1/users/me/api-tokens", map[string]string{"name": name}, &created); err != nil {
		return "", err
	}

	return created.Token, nil
}

func callSMSC(ctx context.Context, method, token, path string, body any, out any) error {
	var reader io.Reader

	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}

		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, smscAPIAddress()+path, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, raw)
	}

	if out == nil {
		return nil
	}

	envelope := struct {
		Result json.RawMessage `json:"result"`
	}{}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}

	return json.Unmarshal(envelope.Result, out)
}

func smscSeesHSS(ctx context.Context, token string) bool {
	var diameter struct {
		Routes []struct {
			Application string   `json:"application"`
			Peers       []string `json:"peers"`
		} `json:"routes"`
	}

	if callSMSC(ctx, http.MethodGet, token, "/api/v1/diameter", nil, &diameter) != nil {
		return false
	}

	for _, r := range diameter.Routes {
		if r.Application == "s6c" && len(r.Peers) > 0 {
			return true
		}
	}

	return false
}
