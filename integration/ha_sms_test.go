// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const (
	haSMSComposeDir  = "compose/ha-3gpp/"
	haSMSComposeFile = "compose.yaml"
	haSMSMarker      = "PHASE1_DONE"
	haSMSSMSCAddress = "10.3.0.5"
	haSMSTesterN2    = "10.100.0.20"
	haSMSTesterN3    = "10.3.0.20"
)

var (
	haSMSServices = []string{"ella-core-1", "ella-core-2", "ella-core-3"}
	haSMSN2       = []string{"10.100.0.11:38412", "10.100.0.12:38412", "10.100.0.13:38412"}
)

func TestIntegrationHASMS(t *testing.T) {
	suites.Require(t, suites.HASMS)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	overlays := []string{"../sms/smsc.yaml"}

	adminToken, nodeClients, err := bringUpHA3GPPCluster(t, ctx, dc, haSMSComposeDir, haSMSComposeFile, bringUpHA3GPPClusterOpts{
		ExtraServices: []string{"ella-core-tester", "smsc"},
		Overlays:      overlays,
	})
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		for _, svc := range append(append([]string{}, haSMSServices...), "smsc") {
			if logs, err := dc.ComposeLogs(cleanupCtx, haSMSComposeDir, svc); err == nil && t.Failed() {
				HALogf(t, "=== %s logs ===\n%s", svc, logs)
			}
		}
	})

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

	if err := haClient.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{
		Enabled:     true,
		SMSCAddress: haSMSSMSCAddress,
		SMSCPort:    smscDiameterPort,
		SMSNumber:   smsNumber,
	}); err != nil {
		t.Fatalf("enable SMS: %v", err)
	}

	hosts, err := waitForClusterSMSCLinks(ctx, nodeClients)
	if err != nil {
		t.Fatal(err)
	}

	leaderIdx, _, err := findLeader(ctx, nodeClients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	order := append([]int{leaderIdx}, others(leaderIdx, len(haSMSServices))...)

	HALogf(t, "node order (leader first): %v, Diameter hosts %v", order, hosts)

	tester, err := dc.ResolveComposeContainer(ctx, "ha-3gpp", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	argv := func(scenario string) []string {
		args := []string{"core-tester", "run", scenario}

		for _, i := range order {
			args = append(args, "--ella-core-n2-address", haSMSN2[i], "--node-host", hosts[i])
		}

		return append(args,
			"--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s", haSMSTesterN2, haSMSTesterN3),
			"--smsc-api-address", "http://"+net.JoinHostPort(haSMSSMSCAddress, strconv.Itoa(smscAPIPort)),
			"--verbose",
		)
	}

	for _, name := range []string{"ha_sms/cross_node_routing", "ha_sms/ue_moves", "ha_sms/absent_then_attach_elsewhere", "ha_sms/node_failure"} {
		sc, ok := scenarios.Get(name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", name))

		t.Run(name, func(t *testing.T) {
			fixture.New(t, ctx, haClient).Apply(sc.Fixture(scenarios.Env{}))

			marker := make(chan struct{})
			done := make(chan error, 1)

			go func() {
				_, err := dc.Exec(ctx, tester, argv(name), false, 5*time.Minute, newMarkerWriter(t, haSMSMarker, marker))
				done <- err
			}()

			select {
			case <-marker:
				victim := haSMSServices[order[1]]
				HALogf(t, "killing the recipient's serving node %s", victim)

				if err := composeKill(ctx, haSMSComposeDir, haSMSComposeFile, victim); err != nil {
					t.Fatalf("kill %s: %v", victim, err)
				}

				if err := <-done; err != nil {
					t.Fatalf("scenario failed: %v", err)
				}
			case err := <-done:
				if err != nil {
					t.Fatalf("scenario failed: %v", err)
				}
			case <-ctx.Done():
				t.Fatalf("scenario did not finish: %v", ctx.Err())
			}
		})
	}
}

func others(skip, n int) []int {
	var out []int

	for i := range n {
		if i != skip {
			out = append(out, i)
		}
	}

	return out
}

func waitForClusterSMSCLinks(ctx context.Context, nodes []*client.Client) ([]string, error) {
	deadline := time.Now().Add(2 * time.Minute)

	for {
		hosts := make([]string, len(nodes))
		open := 0

		for i, c := range nodes {
			status, err := c.GetDiameterStatus(ctx)
			if err != nil {
				continue
			}

			hosts[i] = status.Host

			for _, p := range status.Peers {
				if p.Role == "smsc" && p.State == "open" && status.Host != "" {
					open++
					break
				}
			}
		}

		if open == len(nodes) && smscSeesHSS(ctx) {
			return hosts, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("only %d of %d nodes opened a Diameter link to the SMSC (hosts %v)", open, len(nodes), hosts)
		}

		time.Sleep(time.Second)
	}
}
