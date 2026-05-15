package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestNetworkRulesAndFlowReports drives a matrix of rule shapes
// against a real PDU session and asserts per-flow content for the
// resulting flow reports, once per (icmp, udp, tcp). Cases are
// ordered so consecutive subtests never share the same
// (protocol, direction, action) filter shape — without that, late
// flow exports from one case could leak into the next case's
// snapshot.
//
// In IPv6 mode the "deny all ICMP" cases would also drop Router
// Solicitation / Router Advertisement traffic and break SLAAC, so
// those cases use a remote prefix scoped to the ping target instead.
func TestNetworkRulesAndFlowReports(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	fp := familyParams(DetectIPFamily())

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	for _, protoName := range []string{"icmp", "udp", "tcp"} {
		pp := protocolParams(fp.family, protoName)

		t.Run(protoName, func(t *testing.T) {
			for _, tc := range buildRuleCases(fp, pp) {
				tc := tc

				t.Run(tc.name, func(t *testing.T) {
					runNetworkRuleCase(ctx, t, env, tc, fp, pp)
				})
			}
		})
	}
}

type ruleCase struct {
	name            string
	rule            *client.PolicyRules
	scenario        string
	assertDirection string // "uplink" or "downlink"
	assertAction    string // "allow" or "drop"
}

func buildRuleCases(fp ipFamilyParams, pp probeProtocolParams) []ruleCase {
	pingTargetPrefix := fp.pingDestination + fp.hostPrefix
	denyAllDescription := "deny all " + pp.name
	denyTargetDescription := "deny " + pp.name + " from ping target"
	denyNonMatchDescription := "deny " + pp.name + " from non-matching prefix"

	// In IPv6 the unscoped all-ICMP deny would also block RS/RA and
	// break SLAAC, so we scope it to the ping target instead.
	downlinkDenyAll := &client.PolicyRules{
		Downlink: []client.PolicyRule{{
			Description: denyAllDescription,
			Protocol:    int32(pp.ipProto),
			Action:      "deny",
		}},
	}

	uplinkDenyAll := &client.PolicyRules{
		Uplink: []client.PolicyRule{{
			Description: denyAllDescription,
			Protocol:    int32(pp.ipProto),
			Action:      "deny",
		}},
	}

	downlinkDenyTarget := &client.PolicyRules{
		Downlink: []client.PolicyRule{{
			Description:  denyTargetDescription,
			Protocol:     int32(pp.ipProto),
			RemotePrefix: ptr(pingTargetPrefix),
			Action:       "deny",
		}},
	}

	uplinkDenyTarget := &client.PolicyRules{
		Uplink: []client.PolicyRule{{
			Description:  denyTargetDescription,
			Protocol:     int32(pp.ipProto),
			RemotePrefix: ptr(pingTargetPrefix),
			Action:       "deny",
		}},
	}

	downlinkRule := downlinkDenyAll
	uplinkRule := uplinkDenyAll

	if fp.family == IPv6Only && pp.name == "icmp" {
		downlinkRule = downlinkDenyTarget
		uplinkRule = uplinkDenyTarget
	}

	cases := []ruleCase{
		{
			name:            "downlink_deny",
			rule:            downlinkRule,
			scenario:        fp.scenarioBlocked,
			assertDirection: "downlink",
			assertAction:    "drop",
		},
		{
			name: "deny_prefix_nonmatch",
			rule: &client.PolicyRules{
				Downlink: []client.PolicyRule{{
					Description:  denyNonMatchDescription,
					Protocol:     int32(pp.ipProto),
					RemotePrefix: ptr(fp.nonMatchingPrefix),
					Action:       "deny",
				}},
			},
			scenario:        fp.scenarioAllowed,
			assertDirection: "downlink",
			assertAction:    "allow",
		},
		{
			name:            "uplink_deny",
			rule:            uplinkRule,
			scenario:        fp.scenarioBlocked,
			assertDirection: "uplink",
			assertAction:    "drop",
		},
		{
			name: "precedence_allow_over_deny",
			rule: &client.PolicyRules{
				Downlink: []client.PolicyRule{
					{
						Description: "allow " + pp.name + " (precedence 0)",
						Protocol:    int32(pp.ipProto),
						Action:      "allow",
					},
					{
						Description: "deny " + pp.name + " (precedence 1, shadowed)",
						Protocol:    int32(pp.ipProto),
						Action:      "deny",
					},
				},
			},
			scenario:        fp.scenarioAllowed,
			assertDirection: "downlink",
			assertAction:    "allow",
		},
		{
			name: "deny_prefix_match",
			rule: &client.PolicyRules{
				Downlink: []client.PolicyRule{{
					Description:  "deny " + pp.name + " from ping destination",
					Protocol:     int32(pp.ipProto),
					RemotePrefix: ptr(pingTargetPrefix),
					Action:       "deny",
				}},
			},
			scenario:        fp.scenarioBlocked,
			assertDirection: "downlink",
			assertAction:    "drop",
		},
	}

	if pp.supportsPortRules {
		cases = append(cases, buildPortRuleCases(fp, pp)...)
	}

	return cases
}

// buildPortRuleCases adds three port-gate cases. Only meaningful for
// TCP/UDP. Each case puts the rule on the downlink side and asserts
// the corresponding downlink flow; uplink coverage is already
// exercised by the prefix and protocol-only cases above.
func buildPortRuleCases(fp ipFamilyParams, pp probeProtocolParams) []ruleCase {
	port := int32(responderPort)

	return []ruleCase{
		{
			name: "deny_port_match",
			rule: &client.PolicyRules{
				Downlink: []client.PolicyRule{{
					Description: "deny " + pp.name + " on responder port",
					Protocol:    int32(pp.ipProto),
					PortLow:     port,
					PortHigh:    port,
					Action:      "deny",
				}},
			},
			scenario:        fp.scenarioBlocked,
			assertDirection: "downlink",
			assertAction:    "drop",
		},
		{
			name: "deny_port_nonmatch",
			rule: &client.PolicyRules{
				Downlink: []client.PolicyRule{{
					Description: "deny " + pp.name + " on unused port",
					Protocol:    int32(pp.ipProto),
					PortLow:     9999,
					PortHigh:    9999,
					Action:      "deny",
				}},
			},
			scenario:        fp.scenarioAllowed,
			assertDirection: "downlink",
			assertAction:    "allow",
		},
		{
			name: "deny_port_range_match",
			rule: &client.PolicyRules{
				Downlink: []client.PolicyRule{{
					Description: "deny " + pp.name + " on responder port range",
					Protocol:    int32(pp.ipProto),
					PortLow:     34000,
					PortHigh:    34999,
					Action:      "deny",
				}},
			},
			scenario:        fp.scenarioBlocked,
			assertDirection: "downlink",
			assertAction:    "drop",
		},
	}
}

func runNetworkRuleCase(ctx context.Context, t *testing.T, env *testerEnv, tc ruleCase, fp ipFamilyParams, pp probeProtocolParams) {
	t.Helper()

	sc, ok := scenarios.Get(tc.scenario)
	if !ok {
		t.Fatalf("scenario %q not registered", tc.scenario)
	}

	scenariosEnv := buildScenariosEnv(env)

	var spec scenarios.FixtureSpec
	if sc.Fixture != nil {
		spec = sc.Fixture(scenariosEnv)
	}

	expectedIMSIs := make([]string, len(spec.Subscribers))
	for i, s := range spec.Subscribers {
		expectedIMSIs[i] = s.IMSI
	}

	fx := fixture.New(t, ctx, env.Client)
	fx.Apply(spec)

	setDefaultPolicyRules(ctx, t, env.Client, tc.rule)

	t.Cleanup(func() {
		setDefaultPolicyRules(context.Background(), t, env.Client, &client.PolicyRules{})
	})

	if err := env.Client.ClearFlowReports(ctx); err != nil {
		t.Fatalf("clear flow reports: %v", err)
	}

	scenarioStart := time.Now()

	env.RunScenario(ctx, t, tc.scenario, scenarioRunArgs(tc.scenario, spec, pp)...)

	scenarioEnd := time.Now()

	filter := client.ListFlowReportsParams{
		Protocol:    apiProtocolFilter(pp),
		Direction:   tc.assertDirection,
		Action:      tc.assertAction,
		Source:      apiSourceIPFilter(tc.assertDirection, fp),
		Destination: apiDestinationIPFilter(tc.assertDirection, fp),
		PerPage:     100,
	}

	// UPF only exports a flow once it has been idle for ~30 s, so
	// visibility lags the last packet by up to ~45 s.
	flows := fixture.AssertFlowReports(
		ctx, t, env.Client,
		&filter,
		expectedFlowsContentPredicate(tc.assertDirection, tc.assertAction, expectedIMSIs, fp, pp),
		90*time.Second,
	)

	if pp.bytesPerFlow != nil {
		fixture.AssertEachBytesIs(t, flows, *pp.bytesPerFlow)
	}

	if pp.packetsPerFlow == nil {
		// Log actuals for calibration of kernel-dependent TCP counts.
		for i, f := range flows {
			t.Logf("flow %d (imsi=%s dir=%s action=%s): packets=%d bytes=%d", i, f.SubscriberID, f.Direction, f.Action, f.Packets, f.Bytes)
		}
	}

	fixture.AssertEachTimestampsWithin(t, flows, scenarioStart, scenarioEnd.Add(timestampUpperBuffer))

	t.Logf("%s/%s: %d %s/%s flows", pp.name, tc.name, len(flows), tc.assertDirection, tc.assertAction)
}

// setDefaultPolicyRules replaces rules on the baseline policy, preserving
// other fields. A nil or empty rules value clears the rule set.
func setDefaultPolicyRules(ctx context.Context, t *testing.T, c *client.Client, rules *client.PolicyRules) {
	t.Helper()

	name := scenarios.DefaultPolicyName

	cur, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
	if err != nil {
		t.Fatalf("get policy %q: %v", name, err)
	}

	if err := c.UpdatePolicy(ctx, name, &client.UpdatePolicyOptions{
		ProfileName:         cur.ProfileName,
		SliceName:           cur.SliceName,
		DataNetworkName:     cur.DataNetworkName,
		SessionAmbrUplink:   cur.SessionAmbrUplink,
		SessionAmbrDownlink: cur.SessionAmbrDownlink,
		Var5qi:              cur.Var5qi,
		Arp:                 cur.Arp,
		Rules:               rules,
	}); err != nil {
		t.Fatalf("update policy %q rules: %v", name, err)
	}
}

func ptr[T any](v T) *T { return &v }
