// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"golang.org/x/sync/errgroup"
)

const (
	scaleSubscribers      = 1000
	scaleRadios           = 10
	scaleConcurrency      = 128
	scaleProvisionWorkers = 16
	scaleStartIMSI        = 1017000000001
)

func TestIntegrationScale1k(t *testing.T) {
	suites.Require(t, suites.Scale)

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	provisionScaleSubscribers(ctx, t, env.Client)

	radios := addScaleRadioAddresses(ctx, t, env)

	for _, tc := range []struct{ name, scenario string }{
		{"5g", "gnb/scale"},
		{"4g", "s1enb/scale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := registerScenarioTest(tc.scenario + "/1k")

			args := []string{
				"--ues", strconv.Itoa(scaleSubscribers),
				"--start-imsi", scaleIMSI(0),
				"--concurrency", strconv.Itoa(scaleConcurrency),
				"--probe-every", "50",
				"--timeout", "2m",
			}

			for i, addr := range radios {
				args = append(args, "--gnb", fmt.Sprintf("scale-%s%d,n2=%s,n3=%s", tc.name, i, addr, addr))
			}

			out := env.RunScenario(ctx, t, tc.scenario, tr, args...)

			for line := range strings.SplitSeq(out, "\n") {
				for _, summary := range []string{"deregistration complete\t", "registration complete\t", "user plane probes complete\t", "over capacity rejected\t"} {
					if i := strings.Index(line, summary); i >= 0 {
						t.Log(line[i:])

						break
					}
				}
			}
		})
	}
}

func scaleIMSI(i int) string {
	return fmt.Sprintf("%015d", scaleStartIMSI+i)
}

func provisionScaleSubscribers(ctx context.Context, t *testing.T, c *client.Client) {
	t.Helper()

	start := time.Now()

	var g errgroup.Group

	g.SetLimit(scaleProvisionWorkers)

	for i := range scaleSubscribers {
		g.Go(func() error {
			spec := scenarios.DefaultSubscriberWith(scaleIMSI(i), "")

			return c.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
				Imsi:           spec.IMSI,
				Key:            spec.Key,
				SequenceNumber: spec.SequenceNumber,
				ProfileName:    spec.ProfileName,
				OPc:            spec.OPc,
			})
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("provision %d subscribers: %v", scaleSubscribers, err)
	}

	t.Logf("provisioned %d subscribers in %s", scaleSubscribers, time.Since(start).Round(time.Millisecond))

	t.Cleanup(func() {
		var g errgroup.Group

		g.SetLimit(scaleProvisionWorkers)

		for i := range scaleSubscribers {
			g.Go(func() error {
				return c.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: scaleIMSI(i)})
			})
		}

		if err := g.Wait(); err != nil {
			t.Logf("cleanup: delete scale subscribers: %v", err)
		}
	})
}

func addScaleRadioAddresses(ctx context.Context, t *testing.T, env *testerEnv) []string {
	t.Helper()

	addrs := make([]string, scaleRadios)

	for i := range scaleRadios {
		addrs[i] = fmt.Sprintf("10.3.0.%d", 100+i)

		argv := []string{"ip", "addr", "replace", addrs[i] + "/24", "dev", "n3"}
		if _, err := env.dc.Exec(ctx, env.TesterContainer, argv, false, 30*time.Second, nil); err != nil {
			t.Fatalf("add radio address %s: %v", addrs[i], err)
		}
	}

	return addrs
}
