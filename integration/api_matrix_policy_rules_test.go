package integration_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runPolicyRulesMatrix covers the network rules sub-resource of a Policy.
// Rules are embedded in the Policy payload and returned in full only by
// GetPolicy (ListPolicies omits them).
//
// Update is destructive on rules: omitting the Rules field clears them,
// so callers must re-supply the current rule list to preserve it. This
// matrix locks both the destructive and the preserve paths in.
func runPolicyRulesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	sliceName := apiMatrixName("rules-slice")
	dnName := apiMatrixName("rules-dn")
	profileName := apiMatrixName("rules-profile")
	policyName := apiMatrixName("rules-policy")

	if err := c.CreateSlice(ctx, &client.CreateSliceOptions{Name: sliceName, Sst: 1, Sd: "00abcd"}); err != nil {
		t.Fatalf("create dep slice: %v", err)
	}

	t.Cleanup(func() {
		_ = c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: sliceName})
	})

	if err := c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name:     dnName,
		IPv4Pool: "10.254.0.0/16",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}); err != nil {
		t.Fatalf("create dep data network: %v", err)
	}

	t.Cleanup(func() {
		_ = c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: dnName})
	})

	if err := c.CreateProfile(ctx, &client.CreateProfileOptions{
		Name:           profileName,
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "100 Mbps",
	}); err != nil {
		t.Fatalf("create dep profile: %v", err)
	}

	t.Cleanup(func() {
		_ = c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profileName})
	})

	prefixV4 := "192.0.2.0/24"
	prefixV6 := "2001:db8::/32"

	initialRules := &client.PolicyRules{
		Uplink: []client.PolicyRule{
			{Description: "uplink-allow-v4-https", RemotePrefix: &prefixV4, Protocol: 6, PortLow: 443, PortHigh: 443, Action: "allow"},
			{Description: "uplink-deny-icmp", RemotePrefix: nil, Protocol: 1, PortLow: 0, PortHigh: 0, Action: "deny"},
		},
		Downlink: []client.PolicyRule{
			{Description: "downlink-allow-v6-dns", RemotePrefix: &prefixV6, Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"},
		},
	}

	if err := c.CreatePolicy(ctx, &client.CreatePolicyOptions{
		Name:                policyName,
		ProfileName:         profileName,
		SliceName:           sliceName,
		DataNetworkName:     dnName,
		SessionAmbrUplink:   "50 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 8,
		Rules:               initialRules,
	}); err != nil {
		t.Fatalf("create policy with rules: %v", err)
	}

	t.Cleanup(func() {
		_ = c.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: policyName})
	})

	got, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: policyName})
	if err != nil {
		t.Fatalf("get policy after create: %v", err)
	}

	if !rulesEqual(got.Rules, initialRules) {
		t.Fatalf("post-create rules round-trip mismatch:\ngot  %s\nwant %s", formatRules(got.Rules), formatRules(initialRules))
	}

	t.Run("update_replace", func(t *testing.T) {
		replaced := &client.PolicyRules{
			Uplink: []client.PolicyRule{
				{Description: "uplink-allow-all-tcp", RemotePrefix: nil, Protocol: 6, PortLow: 1, PortHigh: 65535, Action: "allow"},
			},
			Downlink: []client.PolicyRule{
				{Description: "downlink-deny-v4-ssh", RemotePrefix: &prefixV4, Protocol: 6, PortLow: 22, PortHigh: 22, Action: "deny"},
				{Description: "downlink-allow-v6-https", RemotePrefix: &prefixV6, Protocol: 6, PortLow: 443, PortHigh: 443, Action: "allow"},
			},
		}

		if err := updatePolicyRules(ctx, c, policyName, replaced); err != nil {
			t.Fatalf("update rules (replace): %v", err)
		}

		after := mustGetPolicy(ctx, t, c, policyName)
		if !rulesEqual(after.Rules, replaced) {
			t.Fatalf("rules after replace:\ngot  %s\nwant %s", formatRules(after.Rules), formatRules(replaced))
		}
	})

	t.Run("update_clear_nil", func(t *testing.T) {
		if err := updatePolicyRules(ctx, c, policyName, nil); err != nil {
			t.Fatalf("update rules (clear nil): %v", err)
		}

		after := mustGetPolicy(ctx, t, c, policyName)
		if after.Rules != nil {
			t.Fatalf("expected Rules == nil after nil-clear, got %s", formatRules(after.Rules))
		}
	})

	t.Run("update_clear_empty_struct", func(t *testing.T) {
		if err := updatePolicyRules(ctx, c, policyName, initialRules); err != nil {
			t.Fatalf("update rules (re-seed): %v", err)
		}

		if err := updatePolicyRules(ctx, c, policyName, &client.PolicyRules{}); err != nil {
			t.Fatalf("update rules (clear empty): %v", err)
		}

		after := mustGetPolicy(ctx, t, c, policyName)
		if after.Rules != nil {
			t.Fatalf("expected Rules == nil after empty-struct clear, got %s", formatRules(after.Rules))
		}
	})

	t.Run("preserve_when_updating_other_field", func(t *testing.T) {
		if err := updatePolicyRules(ctx, c, policyName, initialRules); err != nil {
			t.Fatalf("update rules (re-seed): %v", err)
		}

		current := mustGetPolicy(ctx, t, c, policyName)
		if current.Rules == nil {
			t.Fatalf("setup: expected rules after re-seed, got nil")
		}

		opts := &client.UpdatePolicyOptions{
			ProfileName:         current.ProfileName,
			SliceName:           current.SliceName,
			DataNetworkName:     current.DataNetworkName,
			SessionAmbrUplink:   current.SessionAmbrUplink,
			SessionAmbrDownlink: current.SessionAmbrDownlink,
			Var5qi:              5,
			Arp:                 current.Arp,
			Rules:               current.Rules,
		}

		if err := c.UpdatePolicy(ctx, policyName, opts); err != nil {
			t.Fatalf("update policy (Var5qi with re-supplied rules): %v", err)
		}

		after := mustGetPolicy(ctx, t, c, policyName)

		if after.Var5qi != 5 {
			t.Fatalf("Var5qi: got %d, want 5", after.Var5qi)
		}

		if !rulesEqual(after.Rules, initialRules) {
			t.Fatalf("rules dropped when updating Var5qi:\ngot  %s\nwant %s", formatRules(after.Rules), formatRules(initialRules))
		}
	})

	negatives := []struct {
		name  string
		rules *client.PolicyRules
		want  string
	}{
		{
			name: "bad_action",
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: "x", Protocol: 6, PortLow: 80, PortHigh: 80, Action: "reject"},
			}},
			want: "action must be 'allow' or 'deny'",
		},
		{
			name: "bad_cidr",
			rules: func() *client.PolicyRules {
				p := "not-a-cidr"

				return &client.PolicyRules{Uplink: []client.PolicyRule{
					{Description: "x", RemotePrefix: &p, Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
				}}
			}(),
			want: "invalid CIDR",
		},
		{
			name: "protocol_out_of_range",
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: "x", Protocol: 999, PortLow: 80, PortHigh: 80, Action: "allow"},
			}},
			want: "protocol must be between 0 and 255",
		},
		{
			name: "port_low_gt_port_high",
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: "x", Protocol: 6, PortLow: 100, PortHigh: 50, Action: "allow"},
			}},
			want: "port_low must be <= port_high",
		},
		{
			name: "description_oversized",
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: strings.Repeat("x", 257), Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			}},
			want: "256 characters or fewer",
		},
		{
			name:  "too_many_per_direction",
			rules: &client.PolicyRules{Uplink: makeNRules(13)},
			want:  "uplink rules exceed maximum",
		},
	}

	// Clear rules first so the negative cases assert "rules unchanged"
	// against a known-empty starting state.
	if err := updatePolicyRules(ctx, c, policyName, nil); err != nil {
		t.Fatalf("setup negatives: clear rules: %v", err)
	}

	for _, n := range negatives {
		n := n
		t.Run("negative_"+n.name, func(t *testing.T) {
			err := updatePolicyRules(ctx, c, policyName, n.rules)
			if err == nil {
				t.Fatalf("expected error, got none")
			}

			msg := err.Error()
			if !strings.Contains(msg, n.want) {
				t.Fatalf("error message: got %q, want substring %q", msg, n.want)
			}

			if !strings.Contains(msg, "400") {
				t.Fatalf("expected 400 status, got %q", msg)
			}

			after := mustGetPolicy(ctx, t, c, policyName)
			if after.Rules != nil {
				t.Fatalf("rules mutated by rejected update %s: %s", n.name, formatRules(after.Rules))
			}
		})
	}
}

// updatePolicyRules issues an Update that changes only Rules, pulling
// the other fields from the current policy state.
func updatePolicyRules(ctx context.Context, c *client.Client, name string, rules *client.PolicyRules) error {
	current, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
	if err != nil {
		return err
	}

	return c.UpdatePolicy(ctx, name, &client.UpdatePolicyOptions{
		ProfileName:         current.ProfileName,
		SliceName:           current.SliceName,
		DataNetworkName:     current.DataNetworkName,
		SessionAmbrUplink:   current.SessionAmbrUplink,
		SessionAmbrDownlink: current.SessionAmbrDownlink,
		Var5qi:              current.Var5qi,
		Arp:                 current.Arp,
		Rules:               rules,
	})
}

func mustGetPolicy(ctx context.Context, t *testing.T, c *client.Client, name string) *client.Policy {
	t.Helper()

	p, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
	if err != nil {
		t.Fatalf("get policy %q: %v", name, err)
	}

	return p
}

func rulesEqual(a, b *client.PolicyRules) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return ruleListEqual(a.Uplink, b.Uplink) && ruleListEqual(a.Downlink, b.Downlink)
}

func ruleListEqual(a, b []client.PolicyRule) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}

func formatRules(r *client.PolicyRules) string {
	if r == nil {
		return "<nil>"
	}

	return "{uplink=" + formatRuleList(r.Uplink) + ", downlink=" + formatRuleList(r.Downlink) + "}"
}

func formatRuleList(rs []client.PolicyRule) string {
	var sb strings.Builder

	sb.WriteByte('[')

	for i, r := range rs {
		if i > 0 {
			sb.WriteString(", ")
		}

		prefix := "<nil>"
		if r.RemotePrefix != nil {
			prefix = *r.RemotePrefix
		}

		sb.WriteString(r.Description + "/" + prefix + "/" + r.Action)
	}

	sb.WriteByte(']')

	return sb.String()
}

func makeNRules(n int) []client.PolicyRule {
	out := make([]client.PolicyRule, n)
	for i := range out {
		out[i] = client.PolicyRule{
			Description: "rule",
			Protocol:    6,
			PortLow:     int32(1000 + i),
			PortHigh:    int32(1000 + i),
			Action:      "allow",
		}
	}

	return out
}
