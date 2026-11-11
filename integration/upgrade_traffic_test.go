// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const upgradeTrafficUEsPerGNB = 5

type upgradeGNB struct {
	service  string
	gnbID    string
	n2       string
	n3       string
	coreN2   string
	imsiBase string
}

var upgradeGNBs = []upgradeGNB{
	{service: "ella-core-tester-1", gnbID: "000001", n2: "10.100.0.21", n3: "10.3.0.21", coreN2: "10.100.0.11:38412", imsiBase: "001019756190100"},
	{service: "ella-core-tester-2", gnbID: "000002", n2: "10.100.0.22", n3: "10.3.0.22", coreN2: "10.100.0.12:38412", imsiBase: "001019756190110"},
	{service: "ella-core-tester-3", gnbID: "000003", n2: "10.100.0.23", n3: "10.3.0.23", coreN2: "10.100.0.13:38412", imsiBase: "001019756190120"},
}

func setUpUpgradeTraffic(t *testing.T, ctx context.Context, dc *DockerClient, composeDir string, clients []*client.Client) {
	t.Helper()

	services := []string{"router"}
	for _, g := range upgradeGNBs[:len(clients)] {
		services = append(services, g.service)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, ComposeFile(), services...); err != nil {
		t.Fatalf("start testers and router: %v", err)
	}

	for i, c := range clients {
		err := c.CreateRoute(ctx, &client.CreateRouteOptions{Destination: "8.8.8.8/32", Gateway: "10.6.0.3", Interface: "n6"})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("create route on node %d: %v", i+1, err)
		}
	}

	c := clients[0]

	if err := c.UpdateSlice(ctx, scenarios.DefaultSliceName, &client.UpdateSliceOptions{Sst: scenarios.DefaultSST, Sd: scenarios.DefaultSD}); err != nil {
		t.Fatalf("update default slice: %v", err)
	}

	if err := c.UpdateProfile(ctx, scenarios.DefaultProfileName, &client.UpdateProfileOptions{
		UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
	}); err != nil {
		t.Fatalf("update default profile: %v", err)
	}

	if err := c.UpdatePolicy(ctx, scenarios.DefaultPolicyName, &client.UpdatePolicyOptions{
		ProfileName: scenarios.DefaultProfileName, SliceName: scenarios.DefaultSliceName, DataNetworkName: scenarios.DefaultDNN,
		SessionAmbrUplink: scenarios.DefaultPolicySessionAmbrUplink, SessionAmbrDownlink: scenarios.DefaultPolicySessionAmbrDownlink,
		Var5qi: 9, Arp: 15,
	}); err != nil {
		t.Fatalf("update default policy: %v", err)
	}

	for _, g := range upgradeGNBs[:len(clients)] {
		for i := 0; i < upgradeTrafficUEsPerGNB; i++ {
			imsi, err := offsetIMSI15(g.imsiBase, i)
			if err != nil {
				t.Fatalf("compute IMSI: %v", err)
			}

			if err := c.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
				Imsi: imsi, Key: scenarios.DefaultKey, OPc: scenarios.DefaultOPC,
				SequenceNumber: scenarios.DefaultSequenceNumber, ProfileName: scenarios.DefaultProfileName,
			}); err != nil {
				t.Fatalf("create traffic subscriber %s: %v", imsi, err)
			}
		}
	}
}

func runUpgradeTraffic(t *testing.T, ctx context.Context, dc *DockerClient, nodes int, when string) {
	t.Helper()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []string
	)

	for _, g := range upgradeGNBs[:nodes] {
		container, err := dc.ResolveComposeContainer(ctx, "ha-rolling", g.service)
		if err != nil {
			t.Fatalf("resolve %s: %v", g.service, err)
		}

		argv := []string{
			"core-tester", "run", "multi/cluster_traffic_5g",
			"--ella-core-n2-address", g.coreN2,
			"--gnb", fmt.Sprintf("gnb%s,n2=%s,n3=%s", g.gnbID, g.n2, g.n3),
			"--ue-count", strconv.Itoa(upgradeTrafficUEsPerGNB),
			"--imsi-base", g.imsiBase,
			"--gnb-id", g.gnbID,
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			out, err := dc.Exec(ctx, container, argv, false, 3*time.Minute, nil)
			if err != nil {
				mu.Lock()

				errs = append(errs, fmt.Sprintf("%s -> %s: %v\n%s", g.service, g.coreN2, err, out))
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("user plane traffic failed %s:\n%s", when, strings.Join(errs, "\n\n"))
	}

	HALogf(t, "user plane traffic passed through %d nodes %s", nodes, when)
}
