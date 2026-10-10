// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const (
	haIMSProject        = "ha-3gpp"
	haIMSPreferred      = 1
	haIMSDefault        = 10
	haIMSTesterN2       = "10.100.0.20"
	haIMSTesterN3       = "10.3.0.20"
	haIMSUEPool         = "10.60.0.0/16"
	haIMSServiceTimeout = 5 * time.Minute
)

var (
	haIMSServices = []string{"ella-core-1", "ella-core-2", "ella-core-3"}
	haIMSN2       = []string{"10.100.0.11:38412", "10.100.0.12:38412", "10.100.0.13:38412"}
	haIMSN3       = []string{"10.3.0.11", "10.3.0.12", "10.3.0.13"}
	haIMSN6       = []string{"10.6.0.11", "10.6.0.12", "10.6.0.13"}
)

type haIMSCase struct {
	name      string
	lost      int
	preferred int
}

func TestIntegrationHAIMS(t *testing.T) {
	suites.Require(t, suites.HAIMS)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	adminToken, nodeClients, err := bringUpHA3GPPCluster(t, ctx, dc, haSMSComposeDir, haSMSComposeFile, bringUpHA3GPPClusterOpts{
		ExtraServices: []string{"ella-core-tester", "ims", "ims-routes"},
		Overlays:      []string{"../ims/ha-ims.yaml"},
		Diameter:      true,
	})
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	haClient, err := client.New(&client.Config{BaseURLs: []string{"http://10.100.0.11:5002", "http://10.100.0.12:5002", "http://10.100.0.13:5002"}})
	if err != nil {
		t.Fatalf("ella HA client: %v", err)
	}

	haClient.SetToken(adminToken)

	fx := fixture.New(t, ctx, haClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	for i, c := range nodeClients {
		if err := c.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: false}); err != nil {
			t.Fatalf("disable NAT on node %d: %v", i+1, err)
		}
	}

	peers, err := addHAIMSCorePeers(ctx, nodeClients)
	if err != nil {
		t.Fatal(err)
	}

	if err := sendIMS(ctx, http.MethodPut, "/api/v1/policy", map[string]string{"interface": "rx"}); err != nil {
		t.Fatalf("set Ella Core as the PCRF of Ella IMS: %v", err)
	}

	tester, err := dc.ResolveComposeContainer(ctx, haIMSProject, "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	imsRoutes, err := dc.ResolveComposeContainer(ctx, haIMSProject, "ims-routes")
	if err != nil {
		t.Fatalf("resolve the IMS routes container: %v", err)
	}

	cases := []haIMSCase{
		{name: "ha_ims/hss_node_loss", lost: 1, preferred: 1},
		{name: "ha_ims/serving_node_loss", lost: 0, preferred: 0},
		{name: "ha_ims/call_across_nodes", lost: 0, preferred: 2},
	}

	for _, tc := range cases {
		sc, ok := scenarios.Get(tc.name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", tc.name))

		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() { dumpHAIMSLogs(t, dc) })

			if err := waitForHAIMSLinks(ctx, nodeClients); err != nil {
				t.Fatal(err)
			}

			leader, _, err := findLeader(ctx, nodeClients)
			if err != nil {
				t.Fatalf("find leader: %v", err)
			}

			rest := others(leader, len(haIMSServices))
			order := []int{rest[0], leader, rest[1]}
			victim := order[tc.lost]

			preferred := order[tc.preferred]

			HALogf(t, "nodes %v (serving first), losing node %d, Ella IMS prefers node %d", order, victim+1, preferred+1)

			if err := preferHAIMSPeer(ctx, peers, preferred); err != nil {
				t.Fatal(err)
			}

			if err := routeIMSToUEsVia(ctx, dc, imsRoutes, order[0]); err != nil {
				t.Fatal(err)
			}

			fixture.New(t, ctx, haClient).Apply(sc.Fixture(scenarios.Env{}))

			argv := []string{"core-tester", "run", tc.name}
			for _, i := range order {
				argv = append(argv, "--ella-core-n2-address", haIMSN2[i])
			}

			argv = append(argv, "--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s", haIMSTesterN2, haIMSTesterN3), "--verbose")

			marker := make(chan struct{})
			done := make(chan error, 1)

			go func() {
				_, err := dc.Exec(ctx, tester, argv, false, haIMSServiceTimeout, newMarkerWriter(t, haSMSMarker, marker))
				done <- err
			}()

			select {
			case <-marker:
				if victim == order[0] {
					if err := routeIMSToUEsVia(ctx, dc, imsRoutes, order[1]); err != nil {
						t.Fatal(err)
					}
				}

				HALogf(t, "killing %s", haIMSServices[victim])

				if err := dc.DisableRestart(ctx, haIMSProject, haIMSServices[victim]); err != nil {
					t.Fatalf("disable restart on %s: %v", haIMSServices[victim], err)
				}

				t.Cleanup(func() { restartHAIMSNode(ctx, t, dc, nodeClients, victim) })

				if err := composeKill(ctx, haSMSComposeDir, haSMSComposeFile, haIMSServices[victim]); err != nil {
					t.Fatalf("kill %s: %v", haIMSServices[victim], err)
				}

				if err := <-done; err != nil {
					t.Fatalf("scenario failed: %v", err)
				}
			case err := <-done:
				t.Fatalf("scenario ended before losing a node: %v", err)
			case <-ctx.Done():
				t.Fatalf("scenario did not finish: %v", ctx.Err())
			}
		})
	}
}

func restartHAIMSNode(ctx context.Context, t *testing.T, dc *DockerClient, nodes []*client.Client, node int) {
	if err := dc.ComposeStartWithFile(ctx, haSMSComposeDir, haSMSComposeFile, haIMSServices[node]); err != nil {
		t.Errorf("restart %s: %v", haIMSServices[node], err)
		return
	}

	if err := waitForAllNodesReady(ctx, nodes); err != nil {
		t.Errorf("nodes not ready after restarting %s: %v", haIMSServices[node], err)
	}
}

func dumpHAIMSLogs(t *testing.T, dc *DockerClient) {
	if !t.Failed() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for _, svc := range append(append([]string{}, haIMSServices...), "ims") {
		if logs, err := dc.ComposeLogs(ctx, haSMSComposeDir, svc); err == nil {
			t.Logf("=== %s logs ===\n%s", svc, logs)
		}
	}
}

type haIMSPeer struct {
	ID     string `json:"id"`
	params map[string]any
}

func addHAIMSCorePeers(ctx context.Context, nodes []*client.Client) ([]haIMSPeer, error) {
	peers := make([]haIMSPeer, len(nodes))

	for i, c := range nodes {
		host, err := waitForDiameterHost(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", i+1, err)
		}

		params := map[string]any{
			"host":         host,
			"address":      haIMSN3[i],
			"port":         coreDiameterPort,
			"transport":    "sctp",
			"applications": []string{"cx", "rx"},
			"priority":     haIMSDefault,
		}

		var created haIMSPeer
		if err := callIMS(ctx, http.MethodPost, "/api/v1/diameter/peers", params, &created); err != nil {
			return nil, fmt.Errorf("add node %d as a peer of Ella IMS: %w", i+1, err)
		}

		created.params = params
		peers[i] = created
	}

	return peers, nil
}

func preferHAIMSPeer(ctx context.Context, peers []haIMSPeer, preferred int) error {
	for i, p := range peers {
		priority := haIMSDefault
		if i == preferred {
			priority = haIMSPreferred
		}

		p.params["priority"] = priority

		if err := callIMS(ctx, http.MethodPut, "/api/v1/diameter/peers/"+p.ID, p.params, nil); err != nil {
			return fmt.Errorf("set the priority of node %d in Ella IMS: %w", i+1, err)
		}
	}

	return nil
}

func routeIMSToUEsVia(ctx context.Context, dc *DockerClient, imsRoutes string, node int) error {
	argv := []string{"ip", "-4", "route", "replace", haIMSUEPool, "via", haIMSN6[node]}

	if _, err := dc.Exec(ctx, imsRoutes, argv, false, 10*time.Second, nil); err != nil {
		return fmt.Errorf("route the UEs from Ella IMS via node %d: %w", node+1, err)
	}

	return nil
}

func waitForDiameterHost(ctx context.Context, c *client.Client) (string, error) {
	deadline := time.Now().Add(2 * time.Minute)

	for {
		status, err := c.GetDiameterStatus(ctx)
		if err == nil && status.Host != "" {
			return status.Host, nil
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("no Diameter identity (status %+v, error %v)", status, err)
		}

		time.Sleep(time.Second)
	}
}

func waitForHAIMSLinks(ctx context.Context, nodes []*client.Client) error {
	for i, c := range nodes {
		if err := waitForCxLink(ctx, c); err != nil {
			return fmt.Errorf("node %d: %w", i+1, err)
		}
	}

	return nil
}

func callIMS(ctx context.Context, method, path string, body, out any) error {
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

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, payload)
	}

	if out == nil {
		return nil
	}

	return json.Unmarshal(payload, &struct {
		Result any `json:"result"`
	}{Result: out})
}
