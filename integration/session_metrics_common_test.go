// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"regexp"
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

var radioLabelsPattern = regexp.MustCompile(`radio="([^"]*)",radio_id="([^"]*)"`)

func radioMetricSeries(t *testing.T, ctx context.Context, c *client.Client, metric, direction string) map[[2]string]float64 {
	t.Helper()

	all, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}

	out := make(map[[2]string]float64)

	for series, v := range all {
		if !strings.HasPrefix(series, metric+"{") || !strings.Contains(series, fmt.Sprintf(`direction=%q`, direction)) {
			continue
		}

		m := radioLabelsPattern.FindStringSubmatch(series)
		if m == nil {
			t.Fatalf("series %s has no radio labels", series)
		}

		out[[2]string{m[2], m[1]}] += v
	}

	return out
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
		radiosBefore := map[string]map[[2]string]float64{}

		for _, dir := range []string{"uplink", "downlink"} {
			labels := append([]string{fmt.Sprintf(`direction=%q`, dir)}, class...)
			before[dir] = [2]float64{
				sessionMetricSeries(t, ctx, env.Client, "app_upf_session_forwarded_packets_total", labels...),
				sessionMetricSeries(t, ctx, env.Client, "app_upf_session_forwarded_bytes_total", labels...),
			}
			radiosBefore[dir] = radioMetricSeries(t, ctx, env.Client, "app_upf_radio_forwarded_packets_total", dir)
		}

		ues := float64(run(t, fp.scenarioAllowed))

		radios, err := env.Client.ListRadios(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list radios: %v", err)
		}

		names := make(map[string]string, len(radios.Items))
		for _, r := range radios.Items {
			names[r.Ref] = r.Name
		}

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

			var identified float64

			for radio, v := range radioMetricSeries(t, ctx, env.Client, "app_upf_radio_forwarded_packets_total", dir) {
				grew := v - radiosBefore[dir][radio]
				if grew == 0 {
					continue
				}

				id, name := radio[0], radio[1]
				if id == "unknown" {
					t.Errorf("%s: %v packets were counted under an unidentified radio", dir, grew)
					continue
				}

				want, ok := names[id]
				if !ok || want != name {
					t.Errorf("%s: radio_id=%q radio=%q, want a radio the API lists under that ref (name %q)", dir, id, name, want)
				}

				identified += grew

				t.Logf("%s: radio %s (%s) +%v packets", dir, id, name, grew)
			}

			if identified < ues*minPackets {
				t.Errorf("%s packets counted per radio grew by %v, want at least %v", dir, identified, ues*minPackets)
			}
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
		radioBefore := sessionMetricSeries(t, ctx, env.Client, "app_upf_radio_dropped_packets_total", `direction="uplink"`, `reason="sdf_filter"`)

		ues := float64(run(t, fp.scenarioBlocked))

		dropped := sessionMetricSeries(t, ctx, env.Client, "app_upf_session_dropped_packets_total", labels...) - before
		if dropped < ues {
			t.Errorf("uplink sdf_filter drops for the default class grew by %v, want at least %v", dropped, ues)
		}

		t.Logf("uplink sdf_filter drops: +%v", dropped)

		radioDropped := sessionMetricSeries(t, ctx, env.Client, "app_upf_radio_dropped_packets_total", `direction="uplink"`, `reason="sdf_filter"`) - radioBefore
		if radioDropped != dropped {
			t.Errorf("uplink sdf_filter drops counted per radio grew by %v, want the same %v as per class", radioDropped, dropped)
		}

		if unknown := sessionMetricSeries(t, ctx, env.Client, "app_upf_radio_dropped_packets_total", `radio_id="unknown"`); unknown != 0 {
			t.Errorf("%v drops were counted under an unidentified radio", unknown)
		}
	})
}
