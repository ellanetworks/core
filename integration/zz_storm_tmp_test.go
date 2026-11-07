// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

func TestTmpRegistrationStorm(t *testing.T) {
	if os.Getenv("ELLA_STORM") == "" {
		t.Skip("set ELLA_STORM")
	}

	ues := soakEnvInt(t, "ELLA_STORM_UES", 600)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatal(err)
	}

	adminToken, nodeClients, err := bringUpHA3GPPCluster(t, ctx, dc, soakComposeDir, bringUpHA3GPPClusterOpts{
		ExtraServices: []string{"ella-core-tester-2", "router"},
		LogLevel:      "info",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		logs, _ := dc.ComposeLogs(context.Background(), soakComposeDir, "ella-core-2")
		_ = os.WriteFile(os.Getenv("ELLA_STORM_LOG"), []byte(logs), 0o644)
		dc.ComposeDownWithFile(context.Background(), soakComposeDir, soakComposeFile)
	})

	haClient, err := client.New(&client.Config{BaseURLs: []string{"http://10.100.0.11:5002", "http://10.100.0.12:5002", "http://10.100.0.13:5002"}})
	if err != nil {
		t.Fatal(err)
	}

	haClient.SetToken(adminToken)

	if err := configureNATAndRoute(ctx, nodeClients); err != nil {
		t.Fatal(err)
	}

	fx := fixture.New(t, ctx, haClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	var subs scenarios.FixtureSpec

	for i := range ues {
		imsi, err := offsetIMSI15("001010000200001", i)
		if err != nil {
			t.Fatal(err)
		}

		subs.Subscribers = append(subs.Subscribers, scenarios.SubscriberSpec{IMSI: imsi, Key: scenarios.DefaultKey, OPc: scenarios.DefaultOPC, SequenceNumber: scenarios.DefaultSequenceNumber, ProfileName: scenarios.DefaultProfileName})
	}

	start := time.Now()
	fx.Apply(subs)
	t.Logf("provisioned %d subscribers in %v", ues, time.Since(start).Round(time.Second))

	container, err := dc.ResolveComposeContainer(ctx, "ha-5g-multi-gnb", "ella-core-tester-2")
	if err != nil {
		t.Fatal(err)
	}

	for _, conc := range []int{ues} {
		out, err := dc.Exec(ctx, container, []string{
			"core-tester", "run", "gnb/scale",
			"--ella-core-n2-address", "10.100.0.12:38412",
			"--gnb", "storm,n2=10.100.0.22,n3=10.3.0.22",
			"--ues", strconv.Itoa(ues), "--start-imsi", "001010000200001",
			"--concurrency", strconv.Itoa(conc), "--probe-every", "0", "--timeout", "90s",
		}, false, 20*time.Minute, nil)

		for line := range strings.SplitSeq(out, "\n") {
			if strings.Contains(line, "complete") || strings.Contains(line, "progress") {
				t.Log(soakTrimLog(line))
			}
		}

		if err != nil {
			t.Logf("scenario error: %v\n%s", err, soakTail(out, 20))
		}
	}
}
