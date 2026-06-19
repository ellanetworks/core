// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// runNetworkRulesAndFlowReports drives a matrix of rule shapes for the given RAN
// (ranPrefix "gnb"/"s1enb"). For each shape, every applicable protocol's probe
// is run back-to-back under a single combined policy, then per-protocol flow
// content is asserted (action/direction/protocol, source/dest CIDR membership,
// exact ICMP/UDP packet+byte counts, bounded TCP per-IMSI counts, timestamps).
// Batching protocols inside a shape amortises the UPF's per-flow flush latency.
// The rule shapes and probe accounting are RAT-agnostic — only the
// connectivity_expect scenario the tester drives differs by RAN.
func runNetworkRulesAndFlowReports(t *testing.T, ranPrefix string) {
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

	for _, shape := range buildRuleShapes(fp) {
		shape := shape

		t.Run(shape.name, func(t *testing.T) {
			runRuleShape(ctx, t, env, fp, shape)
		})
	}
}

// ruleShape describes one row in the matrix. A shape selects a set of
// protocols, builds one combined policy whose rules cover all of them,
// runs each protocol's probe under the same policy, and asserts the
// per-protocol downlink-or-uplink flow content.
type ruleShape struct {
	name      string
	direction string // "uplink" or "downlink"
	action    string // "allow" or "drop"
	scenario  string // "allowed" or "blocked", resolved from fp
	protocols []string
	// buildRules returns the per-protocol rules for one protocol. The
	// returned rules are merged with rules from other protocols and
	// placed into either policy.Downlink or policy.Uplink depending
	// on shape.direction.
	buildRules func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule
}

func buildRuleShapes(fp ipFamilyParams) []ruleShape {
	allProtos := []string{"icmp", "udp", "tcp"}
	portProtos := []string{"udp", "tcp"}

	pingTargetPrefix := fp.pingDestination + fp.hostPrefix

	return []ruleShape{
		{
			name:      "downlink_deny",
			direction: "downlink",
			action:    "drop",
			scenario:  fp.scenarioBlocked,
			protocols: allProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				// IPv6 ICMP unscoped deny would also drop RS/RA and
				// break SLAAC; scope it to the ping target instead.
				if fp.family == IPv6Only && pp.name == "icmp" {
					return []client.PolicyRule{{
						Description:  "deny " + pp.name + " from ping target",
						Protocol:     int32(pp.ipProto),
						RemotePrefix: ptr(pingTargetPrefix),
						Action:       "deny",
					}}
				}

				return []client.PolicyRule{{
					Description: "deny all " + pp.name,
					Protocol:    int32(pp.ipProto),
					Action:      "deny",
				}}
			},
		},
		{
			name:      "deny_prefix_nonmatch",
			direction: "downlink",
			action:    "allow",
			scenario:  fp.scenarioAllowed,
			protocols: allProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				return []client.PolicyRule{{
					Description:  "deny " + pp.name + " from non-matching prefix",
					Protocol:     int32(pp.ipProto),
					RemotePrefix: ptr(fp.nonMatchingPrefix),
					Action:       "deny",
				}}
			},
		},
		{
			name:      "uplink_deny",
			direction: "uplink",
			action:    "drop",
			scenario:  fp.scenarioBlocked,
			protocols: allProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				if fp.family == IPv6Only && pp.name == "icmp" {
					return []client.PolicyRule{{
						Description:  "deny " + pp.name + " from ping target",
						Protocol:     int32(pp.ipProto),
						RemotePrefix: ptr(pingTargetPrefix),
						Action:       "deny",
					}}
				}

				return []client.PolicyRule{{
					Description: "deny all " + pp.name,
					Protocol:    int32(pp.ipProto),
					Action:      "deny",
				}}
			},
		},
		{
			name:      "precedence_allow_over_deny",
			direction: "downlink",
			action:    "allow",
			scenario:  fp.scenarioAllowed,
			protocols: allProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				return []client.PolicyRule{
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
				}
			},
		},
		{
			name:      "deny_prefix_match",
			direction: "downlink",
			action:    "drop",
			scenario:  fp.scenarioBlocked,
			protocols: allProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				return []client.PolicyRule{{
					Description:  "deny " + pp.name + " from ping destination",
					Protocol:     int32(pp.ipProto),
					RemotePrefix: ptr(pingTargetPrefix),
					Action:       "deny",
				}}
			},
		},
		{
			name:      "deny_port_match",
			direction: "downlink",
			action:    "drop",
			scenario:  fp.scenarioBlocked,
			protocols: portProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				port := int32(responderPort)

				return []client.PolicyRule{{
					Description: "deny " + pp.name + " on responder port",
					Protocol:    int32(pp.ipProto),
					PortLow:     port,
					PortHigh:    port,
					Action:      "deny",
				}}
			},
		},
		{
			name:      "deny_port_nonmatch",
			direction: "downlink",
			action:    "allow",
			scenario:  fp.scenarioAllowed,
			protocols: portProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				return []client.PolicyRule{{
					Description: "deny " + pp.name + " on unused port",
					Protocol:    int32(pp.ipProto),
					PortLow:     9999,
					PortHigh:    9999,
					Action:      "deny",
				}}
			},
		},
		{
			name:      "deny_port_range_match",
			direction: "downlink",
			action:    "drop",
			scenario:  fp.scenarioBlocked,
			protocols: portProtos,
			buildRules: func(fp ipFamilyParams, pp probeProtocolParams) []client.PolicyRule {
				return []client.PolicyRule{{
					Description: "deny " + pp.name + " on responder port range",
					Protocol:    int32(pp.ipProto),
					PortLow:     34000,
					PortHigh:    34999,
					Action:      "deny",
				}}
			},
		},
	}
}

func runRuleShape(ctx context.Context, t *testing.T, env *testerEnv, fp ipFamilyParams, shape ruleShape) {
	t.Helper()

	sc, ok := scenarios.Get(shape.scenario)
	Assert(t, ok, fmt.Sprintf("scenario %q not registered", shape.scenario))

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

	pps := make(map[string]probeProtocolParams, len(shape.protocols))

	policy := &client.PolicyRules{}

	for _, p := range shape.protocols {
		pp := protocolParams(fp.family, p)
		pps[p] = pp

		rules := shape.buildRules(fp, pp)
		switch shape.direction {
		case "downlink":
			policy.Downlink = append(policy.Downlink, rules...)
		case "uplink":
			policy.Uplink = append(policy.Uplink, rules...)
		default:
			t.Fatalf("invalid shape direction %q", shape.direction)
		}
	}

	setDefaultPolicyRules(ctx, t, env.Client, policy)

	t.Cleanup(func() {
		setDefaultPolicyRules(context.Background(), t, env.Client, &client.PolicyRules{})
	})

	if err := env.Client.ClearFlowReports(ctx); err != nil {
		t.Fatalf("clear flow reports: %v", err)
	}

	scenarioStart := time.Now()

	for _, p := range shape.protocols {
		pp := pps[p]
		scenarioName := fmt.Sprintf("%s/%s", shape.scenario, p)
		tr := globalReporter.Start(scenarioName)
		QuietLogf(t, tr, "running %s", scenarioName)
		env.RunScenario(ctx, t, shape.scenario, tr, scenarioRunArgs(shape.scenario, spec, pp)...)
		finishScenarioTest(t, tr)
	}

	scenarioEnd := time.Now()

	for _, p := range shape.protocols {
		pp := pps[p]
		t.Run(p, func(t *testing.T) {
			assertRuleShapeProtocol(ctx, t, env, fp, pp, shape, expectedIMSIs, scenarioStart, scenarioEnd)
		})
	}
}

func assertRuleShapeProtocol(
	ctx context.Context,
	t *testing.T,
	env *testerEnv,
	fp ipFamilyParams,
	pp probeProtocolParams,
	shape ruleShape,
	expectedIMSIs []string,
	scenarioStart, scenarioEnd time.Time,
) {
	t.Helper()

	filter := client.ListFlowReportsParams{
		Protocol:    apiProtocolFilter(pp),
		Direction:   shape.direction,
		Action:      shape.action,
		Source:      apiSourceIPFilter(shape.direction, fp),
		Destination: apiDestinationIPFilter(shape.direction, fp),
		PerPage:     100,
	}

	flows := fixture.AssertFlowReports(
		ctx, t, env.Client,
		&filter,
		expectedFlowsContentPredicate(shape.direction, shape.action, expectedIMSIs, fp, pp),
		90*time.Second,
	)

	if b := expectedBytesPerFlow(pp, shape.direction); b != nil {
		fixture.AssertEachBytesIs(t, flows, *b)
	}

	if pp.packetsPerFlow == nil {
		for i, f := range flows {
			t.Logf("flow %d (imsi=%s dir=%s action=%s): packets=%d bytes=%d", i, f.SubscriberID, f.Direction, f.Action, f.Packets, f.Bytes)
		}
	}

	fixture.AssertEachTimestampsWithin(t, flows, scenarioStart, scenarioEnd.Add(timestampUpperBuffer))

	t.Logf("%s: %d %s/%s flows", pp.name, len(flows), shape.direction, shape.action)
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
