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
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const (
	imsScenarioPrefix = "ims/"
	imsAPIPort        = 5020
	coreDiameterPort  = 3868
)

func imsAPIAddress() string {
	address := "10.3.0.6"
	if DetectIPFamily() != IPv4Only {
		address = "2001:db8:1::16"
	}

	return "http://" + net.JoinHostPort(address, strconv.Itoa(imsAPIPort))
}

func imsOverlay() string {
	if DetectIPFamily() == IPv4Only {
		return "../ims/ims.yaml"
	}

	return "../ims/ims-ipv6.yaml"
}

func TestIntegrationIMS(t *testing.T) {
	suites.Require(t, suites.IMS)

	ctx := context.Background()
	env := setupTesterEnv(ctx, t, imsOverlay())

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	requireOpenIMSPeerPolicy(ctx, t, env.Client)

	imsHost := allowEllaIMS(ctx, t, env.Client)

	addIMSCorePeer(ctx, t, env.Client)

	if err := waitForIMSPeerLink(ctx, env.Client, imsHost, ellaIMSPeerName); err != nil {
		t.Fatal(err)
	}

	requireUnknownPeerRejected(ctx, t, env.Client)
	requireIMSPeerRemovalDisconnects(ctx, t, env.Client, imsHost)

	if err := sendIMS(ctx, http.MethodPut, "/api/v1/policy", map[string]string{"interface": "rx"}); err != nil {
		t.Fatalf("set Ella Core as the PCRF of Ella IMS: %v", err)
	}

	for _, name := range scenarios.List() {
		if !strings.HasPrefix(name, imsScenarioPrefix) {
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

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(sc.Fixture(scenarios.Env{}))

			env.RunScenario(ctx, t, name, tr, "--ip-version", string(DetectIPFamily()))
		})
	}

	t.Run("home_domain", func(t *testing.T) {
		runIMSHomeDomainChecks(ctx, t, env.Client, imsHost)
	})

	printTesterSummary(t)
}

func addIMSCorePeer(ctx context.Context, t *testing.T, cl *client.Client, applications ...string) {
	t.Helper()

	status, err := cl.GetDiameterStatus(ctx)
	if err != nil {
		t.Fatalf("get the Diameter status: %v", err)
	}

	peer := map[string]any{
		"host":         status.LocalOriginHost(),
		"address":      N2Address(0),
		"port":         coreDiameterPort,
		"transport":    "sctp",
		"applications": append([]string{"cx", "rx"}, applications...),
	}

	deadline := time.Now().Add(time.Minute)

	for {
		err := postIMS(ctx, "/api/v1/diameter/peers", peer)
		if err == nil {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("add Ella Core as the HSS and PCRF of Ella IMS: %v", err)
		}

		time.Sleep(time.Second)
	}
}

func waitForCxLink(ctx context.Context, cl *client.Client) error {
	deadline := time.Now().Add(2 * time.Minute)

	for {
		status, err := cl.GetDiameterStatus(ctx)
		if err == nil {
			for _, p := range status.Peers {
				if p.Direction == "inbound" && p.State == "open" {
					return nil
				}
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("Ella IMS did not open a Diameter connection to Ella Core (status %+v, error %v)", status, err)
		}

		time.Sleep(time.Second)
	}
}

func postIMS(ctx context.Context, path string, body any) error {
	return sendIMS(ctx, http.MethodPost, path, body)
}

func sendIMS(ctx context.Context, method, path string, body any) error {
	return callIMS(ctx, method, path, body, nil)
}

// imsAdmin is the admin of Ella IMS that the tests initialize it with.
var imsAdmin = map[string]string{"email": "admin@ellanetworks.com", "password": "integration-test"}

// imsToken is the access token of the IMS API, set by the first call; empty to log in again.
var imsToken struct {
	sync.Mutex
	token string
}

// callIMS sends body, if any, to the IMS API, and decodes the result of the response into out, if not nil. It logs
// in on the first call, initializing Ella IMS if no user exists, and again when its token is no longer accepted: it
// lasts 15 minutes, and each test starts a new Ella IMS.
func callIMS(ctx context.Context, method, path string, body, out any) error {
	token, err := imsLogin(ctx, false)
	if err != nil {
		return err
	}

	status, payload, err := doIMS(ctx, method, path, token, body)
	if err == nil && status == http.StatusUnauthorized {
		if token, err = imsLogin(ctx, true); err != nil {
			return err
		}

		status, payload, err = doIMS(ctx, method, path, token, body)
	}

	if err != nil {
		return err
	}

	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("%s %s: %s: %s", method, path, http.StatusText(status), payload)
	}

	if out == nil {
		return nil
	}

	return json.Unmarshal(payload, &struct {
		Result any `json:"result"`
	}{Result: out})
}

// imsLogin returns the access token of the IMS API, logging in if there is none or renew is set.
func imsLogin(ctx context.Context, renew bool) (string, error) {
	imsToken.Lock()
	defer imsToken.Unlock()

	if imsToken.token != "" && !renew {
		return imsToken.token, nil
	}

	imsToken.token = ""

	status, payload, err := doIMS(ctx, http.MethodPost, "/api/v1/init", "", imsAdmin)
	if err == nil && status == http.StatusForbidden {
		status, payload, err = doIMS(ctx, http.MethodPost, "/api/v1/auth/login", "", imsAdmin)
	}

	if err != nil {
		return "", fmt.Errorf("log in to Ella IMS: %w", err)
	}

	if status != http.StatusOK && status != http.StatusCreated {
		return "", fmt.Errorf("log in to Ella IMS: %s: %s", http.StatusText(status), payload)
	}

	var res struct {
		Result struct {
			Token string `json:"token"`
		} `json:"result"`
	}

	if err := json.Unmarshal(payload, &res); err != nil || res.Result.Token == "" {
		return "", fmt.Errorf("log in to Ella IMS: no token in %s", payload)
	}

	imsToken.token = res.Result.Token

	return imsToken.token, nil
}

// doIMS sends body, if any, to the IMS API with token, if any, and returns the status and the body of the response.
func doIMS(ctx context.Context, method, path, token string, body any) (int, []byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, imsAPIAddress()+path, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, payload, nil
}
