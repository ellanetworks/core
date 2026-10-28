// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const sessionMetricsProbeSourcePortBase = 23000

func sessionMetricSeries(t *testing.T, ctx context.Context, c *client.Client, metric string, labels ...string) float64 {
	t.Helper()

	all, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}

	var sum float64

	for series, v := range all {
		if !strings.HasPrefix(series, metric+"{") {
			continue
		}

		matches := true

		for _, l := range labels {
			if !strings.Contains(series, l) {
				matches = false
				break
			}
		}

		if matches {
			sum += v
		}
	}

	return sum
}

func runSessionMetrics(t *testing.T, ranPrefix string) {
	t.Helper()

	fp := familyParams(DetectIPFamily(), ranPrefix)

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	class := []string{
		fmt.Sprintf(`dnn=%q`, scenarios.DefaultDNN),
		fmt.Sprintf(`sst="%d"`, scenarios.DefaultSST),
		`fiveqi="9"`,
	}

	pp := protocolParams(fp.family, "icmp")
	minPackets := float64(probeRoundtrips)

	run := func(t *testing.T, scenario string) int {
		t.Helper()

		sc, ok := scenarios.Get(scenario)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", scenario))

		var spec scenarios.FixtureSpec
		if sc.Fixture != nil {
			spec = sc.Fixture(buildScenariosEnv(env))
		}

		fixture.New(t, ctx, env.Client).Apply(spec)

		tr := globalReporter.Start(scenario + "/icmp")
		env.RunScenario(ctx, t, scenario, tr, scenarioRunArgs(scenario, spec, pp, sessionMetricsProbeSourcePortBase)...)
		finishScenarioTest(t, tr)

		return len(spec.Subscribers)
	}

	t.Run("forwarded", func(t *testing.T) {
		before := map[string][2]float64{}

		for _, dir := range []string{"uplink", "downlink"} {
			labels := append([]string{fmt.Sprintf(`direction=%q`, dir)}, class...)
			before[dir] = [2]float64{
				sessionMetricSeries(t, ctx, env.Client, "app_upf_session_forwarded_packets_total", labels...),
				sessionMetricSeries(t, ctx, env.Client, "app_upf_session_forwarded_bytes_total", labels...),
			}
		}

		ues := float64(run(t, fp.scenarioAllowed))

		for _, dir := range []string{"uplink", "downlink"} {
			labels := append([]string{fmt.Sprintf(`direction=%q`, dir)}, class...)
			packets := sessionMetricSeries(t, ctx, env.Client, "app_upf_session_forwarded_packets_total", labels...) - before[dir][0]
			bytes := sessionMetricSeries(t, ctx, env.Client, "app_upf_session_forwarded_bytes_total", labels...) - before[dir][1]

			if packets < ues*minPackets {
				t.Errorf("%s packets for the default class grew by %v, want at least %v", dir, packets, ues*minPackets)
			}

			if bytes < packets*20 {
				t.Errorf("%s bytes for the default class grew by %v over %v packets, want at least an IP header per packet", dir, bytes, packets)
			}

			t.Logf("%s: +%v packets, +%v bytes", dir, packets, bytes)
		}
	})

	t.Run("dropped", func(t *testing.T) {
		setDefaultPolicyRules(ctx, t, env.Client, &client.PolicyRules{
			Uplink: []client.PolicyRule{{
				Description:  "deny icmp to the ping target",
				Protocol:     int32(pp.ipProto),
				RemotePrefix: ptr(fp.pingDestination + fp.hostPrefix),
				Action:       "deny",
			}},
		})

		t.Cleanup(func() {
			setDefaultPolicyRules(context.Background(), t, env.Client, &client.PolicyRules{})
		})

		time.Sleep(rulePropagationDelay)

		labels := append([]string{`direction="uplink"`, `reason="sdf_filter"`}, class...)
		before := sessionMetricSeries(t, ctx, env.Client, "app_upf_session_dropped_packets_total", labels...)

		ues := float64(run(t, fp.scenarioBlocked))

		dropped := sessionMetricSeries(t, ctx, env.Client, "app_upf_session_dropped_packets_total", labels...) - before
		if dropped < ues {
			t.Errorf("uplink sdf_filter drops for the default class grew by %v, want at least %v", dropped, ues)
		}

		t.Logf("uplink sdf_filter drops: +%v", dropped)
	})
}
