// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// rulePropagationDelay lets the UPF's changefeed reconciler register a policy's
// SDF filters before a session establishes and binds its filter index.
const rulePropagationDelay = 3 * time.Second

// ipProtoNumber maps a probe protocol name to its IP protocol number for an
// SDF filter rule.
func ipProtoNumber(proto string) int32 {
	switch proto {
	case "icmp":
		return int32(ipProtoICMP)
	case "tcp":
		return int32(ipProtoTCP)
	case "udp":
		return int32(ipProtoUDP)
	default:
		return 0
	}
}

// ruleShape4G describes one network-rule case: a policy rule installed on the
// default policy, the 4G scenario that asserts the resulting enforcement, and
// the protocols it applies to.
type ruleShape4G struct {
	name      string
	scenario  string // s1enb/connectivity_expect_allowed | _blocked
	protocols []string
	buildRule func(proto string) client.PolicyRule
}

// TestIntegration4GNetworkRules installs network (firewall) rules on the policy and drives a
// 4G UE through s1enb/connectivity_expect_{allowed,blocked} to assert the UPF
// enforces each rule over an EPS bearer: protocol, remote-prefix, port, and
// allow-over-deny precedence matching. The UPF rule engine is RAT-agnostic; this
// proves it applies to the 4G data path the same as 5G.
func TestIntegration4GNetworkRules(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skipf("TestIntegration4GNetworkRules runs in IPv4 mode, current %s", DetectIPFamily())
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	// The two subscribers the expect-allowed/blocked scenarios attach. Provisioned
	// once here (the scenarios reuse them across every rule shape). The IMSIs must
	// match netRuleAllowedIMSI/netRuleBlockedIMSI in the s1enb scenario.
	baseline.Apply(scenarios.FixtureSpec{Subscribers: []scenarios.SubscriberSpec{
		scenarios.DefaultSubscriberWith("001017271246610", ""),
		scenarios.DefaultSubscriberWith("001017271246611", ""),
	}})

	const (
		allowed = "s1enb/connectivity_expect_allowed"
		blocked = "s1enb/connectivity_expect_blocked"
	)

	pingPrefix := scenarios.DefaultPingDestination + "/32"
	nonMatchingPrefix := "203.0.113.0/24"
	port := int32(scenarios.DefaultProbePort)

	shapes := []ruleShape4G{
		{
			name: "deny_all_blocks", scenario: blocked, protocols: []string{"icmp", "tcp", "udp"},
			buildRule: func(proto string) client.PolicyRule {
				return client.PolicyRule{Description: "deny all " + proto, Protocol: ipProtoNumber(proto), Action: "deny"}
			},
		},
		{
			name: "deny_nonmatching_prefix_allows", scenario: allowed, protocols: []string{"icmp", "tcp", "udp"},
			buildRule: func(proto string) client.PolicyRule {
				return client.PolicyRule{Description: "deny " + proto + " from non-matching prefix", Protocol: ipProtoNumber(proto), RemotePrefix: ptr(nonMatchingPrefix), Action: "deny"}
			},
		},
		{
			name: "deny_matching_prefix_blocks", scenario: blocked, protocols: []string{"icmp", "tcp", "udp"},
			buildRule: func(proto string) client.PolicyRule {
				return client.PolicyRule{Description: "deny " + proto + " from ping destination", Protocol: ipProtoNumber(proto), RemotePrefix: ptr(pingPrefix), Action: "deny"}
			},
		},
		{
			name: "deny_matching_port_blocks", scenario: blocked, protocols: []string{"tcp", "udp"},
			buildRule: func(proto string) client.PolicyRule {
				return client.PolicyRule{Description: "deny " + proto + " on responder port", Protocol: ipProtoNumber(proto), PortLow: port, PortHigh: port, Action: "deny"}
			},
		},
		{
			name: "deny_nonmatching_port_allows", scenario: allowed, protocols: []string{"tcp", "udp"},
			buildRule: func(proto string) client.PolicyRule {
				return client.PolicyRule{Description: "deny " + proto + " on unused port", Protocol: ipProtoNumber(proto), PortLow: 9999, PortHigh: 9999, Action: "deny"}
			},
		},
	}

	for _, shape := range shapes {
		shape := shape

		t.Run(shape.name, func(t *testing.T) {
			runRuleShape4G(ctx, t, env, shape)
		})
	}

	// precedence: an allow rule shadows a lower-priority deny on the same match,
	// so traffic is permitted (TS 23.503 rule precedence).
	t.Run("precedence_allow_over_deny_allows", func(t *testing.T) {
		policy := &client.PolicyRules{Downlink: []client.PolicyRule{
			{Description: "allow icmp (precedence 0)", Protocol: int32(ipProtoICMP), Action: "allow"},
			{Description: "deny icmp (precedence 1, shadowed)", Protocol: int32(ipProtoICMP), Action: "deny"},
		}}
		setDefaultPolicyRules(ctx, t, env.Client, policy)
		t.Cleanup(func() { setDefaultPolicyRules(context.Background(), t, env.Client, &client.PolicyRules{}) })
		time.Sleep(rulePropagationDelay)

		applyAndRun(ctx, t, env, allowed, "icmp")
	})
}

func runRuleShape4G(ctx context.Context, t *testing.T, env *testerEnv, shape ruleShape4G) {
	t.Helper()

	policy := &client.PolicyRules{}
	for _, proto := range shape.protocols {
		policy.Downlink = append(policy.Downlink, shape.buildRule(proto))
	}

	setDefaultPolicyRules(ctx, t, env.Client, policy)
	t.Cleanup(func() { setDefaultPolicyRules(context.Background(), t, env.Client, &client.PolicyRules{}) })

	// The UPF registers filters asynchronously via the changefeed reconciler, so
	// let the rule reach the data path before a session establishes (which binds
	// its SDF filter index at establish time).
	time.Sleep(rulePropagationDelay)

	for _, proto := range shape.protocols {
		applyAndRun(ctx, t, env, shape.scenario, proto)
	}
}

// applyAndRun runs the scenario for one protocol; the scenario itself fails the
// run when enforcement does not match its expectation. Subscribers are
// provisioned once by the caller.
func applyAndRun(ctx context.Context, t *testing.T, env *testerEnv, scenario, proto string) {
	t.Helper()

	name := fmt.Sprintf("%s/%s", scenario, proto)
	tr := globalReporter.Start(name)
	QuietLogf(t, tr, "running %s", name)
	env.RunScenario(ctx, t, scenario, tr, "--protocol", proto)
	finishScenarioTest(t, tr)
}
