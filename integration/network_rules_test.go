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

// TestNetworkRulesAndFlowReports drives a matrix of rule shapes against
// a real PDU session and asserts per-flow content for the resulting
// flow reports. Cases are ordered so consecutive subtests never share
// the same (protocol, direction, action) filter — without that, late
// flow exports from one case could leak into the next case's snapshot.
//
// In IPv6 mode the "deny all ICMP" cases would also drop Router
// Solicitation / Router Advertisement traffic and break SLAAC, so those
// cases use a remote prefix scoped to the ping target instead.
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

	cases := buildRuleCases(fp)

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			runNetworkRuleCase(ctx, t, env, tc, fp)
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

func buildRuleCases(fp ipFamilyParams) []ruleCase {
	pingTargetPrefix := fp.pingDestination + fp.hostPrefix
	denyAllDescription := "deny all icmp"
	denyTargetDescription := "deny icmp from ping target"
	denyNonMatchDescription := "deny icmp from non-matching prefix"

	// In IPv6 the unscoped all-ICMP deny would also block RS/RA and
	// break SLAAC, so we scope it to the ping target instead.
	downlinkDenyAll := &client.PolicyRules{
		Downlink: []client.PolicyRule{{
			Description: denyAllDescription,
			Protocol:    fp.ruleProtocol,
			Action:      "deny",
		}},
	}

	uplinkDenyAll := &client.PolicyRules{
		Uplink: []client.PolicyRule{{
			Description: denyAllDescription,
			Protocol:    fp.ruleProtocol,
			Action:      "deny",
		}},
	}

	downlinkDenyTarget := &client.PolicyRules{
		Downlink: []client.PolicyRule{{
			Description:  denyTargetDescription,
			Protocol:     fp.ruleProtocol,
			RemotePrefix: ptr(pingTargetPrefix),
			Action:       "deny",
		}},
	}

	uplinkDenyTarget := &client.PolicyRules{
		Uplink: []client.PolicyRule{{
			Description:  denyTargetDescription,
			Protocol:     fp.ruleProtocol,
			RemotePrefix: ptr(pingTargetPrefix),
			Action:       "deny",
		}},
	}

	downlinkRule := downlinkDenyAll
	uplinkRule := uplinkDenyAll

	if fp.family == IPv6Only {
		downlinkRule = downlinkDenyTarget
		uplinkRule = uplinkDenyTarget
	}

	return []ruleCase{
		{
			name:            "downlink_deny_icmp",
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
					Protocol:     fp.ruleProtocol,
					RemotePrefix: ptr(fp.nonMatchingPrefix),
					Action:       "deny",
				}},
			},
			scenario:        fp.scenarioAllowed,
			assertDirection: "downlink",
			assertAction:    "allow",
		},
		{
			name:            "uplink_deny_icmp",
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
						Description: "allow icmp (precedence 0)",
						Protocol:    fp.ruleProtocol,
						Action:      "allow",
					},
					{
						Description: "deny icmp (precedence 1, shadowed)",
						Protocol:    fp.ruleProtocol,
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
					Description:  "deny icmp from ping destination",
					Protocol:     fp.ruleProtocol,
					RemotePrefix: ptr(pingTargetPrefix),
					Action:       "deny",
				}},
			},
			scenario:        fp.scenarioBlocked,
			assertDirection: "downlink",
			assertAction:    "drop",
		},
	}
}

func runNetworkRuleCase(ctx context.Context, t *testing.T, env *testerEnv, tc ruleCase, fp ipFamilyParams) {
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

	env.RunScenario(ctx, t, tc.scenario, scenarioRunArgs(tc.scenario, spec)...)

	scenarioEnd := time.Now()

	filter := client.ListFlowReportsParams{
		Protocol:    fp.protocolFilter,
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
		expectedFlowsContentPredicate(tc.assertDirection, tc.assertAction, expectedIMSIs, fp),
		90*time.Second,
	)

	fixture.AssertEachBytesIs(t, flows, expectedBytesPerFlow(fp))
	fixture.AssertEachTimestampsWithin(t, flows, scenarioStart, scenarioEnd.Add(timestampUpperBuffer))

	t.Logf("%s: %d %s/%s flows", tc.name, len(flows), tc.assertDirection, tc.assertAction)
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
