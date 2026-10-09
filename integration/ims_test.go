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

	if err := env.Client.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: false}); err != nil {
		t.Fatalf("disable NAT: %v", err)
	}

	addIMSCorePeer(ctx, t, env.Client)

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

	printTesterSummary(t)
}

func addIMSCorePeer(ctx context.Context, t *testing.T, cl *client.Client) {
	t.Helper()

	status, err := cl.GetDiameterStatus(ctx)
	if err != nil {
		t.Fatalf("get the Diameter status: %v", err)
	}

	peer := map[string]any{
		"host":         status.Host,
		"realm":        status.Realm,
		"address":      N2Address(0),
		"port":         coreDiameterPort,
		"transport":    "sctp",
		"applications": []string{"cx", "rx"},
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
				if p.Role == "ims" && p.State == "open" {
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
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, imsAPIAddress()+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, msg)
	}

	return nil
}
