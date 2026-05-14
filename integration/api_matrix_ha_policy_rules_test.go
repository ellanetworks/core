package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runPolicyRulesHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	leader := h.Leader

	sliceName := apiMatrixName("ha-rules-slice")
	dnName := apiMatrixName("ha-rules-dn")
	profileName := apiMatrixName("ha-rules-profile")
	policyName := apiMatrixName("ha-rules-policy")

	if err := leader.CreateSlice(ctx, &client.CreateSliceOptions{Name: sliceName, Sst: 1, Sd: "00abcd"}); err != nil {
		t.Fatalf("create dep slice: %v", err)
	}

	t.Cleanup(func() {
		_ = leader.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: sliceName})
	})

	if err := leader.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name:     dnName,
		IPv4Pool: "10.254.0.0/16",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}); err != nil {
		t.Fatalf("create dep data network: %v", err)
	}

	t.Cleanup(func() {
		_ = leader.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: dnName})
	})

	if err := leader.CreateProfile(ctx, &client.CreateProfileOptions{
		Name:           profileName,
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "100 Mbps",
	}); err != nil {
		t.Fatalf("create dep profile: %v", err)
	}

	t.Cleanup(func() {
		_ = leader.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profileName})
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

	if err := nodes[0].CreatePolicy(ctx, &client.CreatePolicyOptions{
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
		t.Fatalf("create policy with rules on node 1: %v", err)
	}

	t.Cleanup(func() {
		_ = leader.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: policyName})
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: policyName})
		if err != nil {
			t.Fatalf("node %d get after create: %v", i+1, err)
		}

		if !rulesEqual(got.Rules, initialRules) {
			t.Fatalf("node %d post-create rules mismatch:\ngot  %s\nwant %s",
				i+1, formatRules(got.Rules), formatRules(initialRules))
		}
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

		if err := updatePolicyRulesOn(ctx, nodes[1], policyName, replaced); err != nil {
			t.Fatalf("update rules (replace) on node 2: %v", err)
		}

		awaitConvergence(ctx, t, h)

		for i, c := range nodes {
			got := mustGetPolicy(ctx, t, c, policyName)
			if !rulesEqual(got.Rules, replaced) {
				t.Fatalf("node %d rules after replace:\ngot  %s\nwant %s",
					i+1, formatRules(got.Rules), formatRules(replaced))
			}
		}
	})

	t.Run("update_clear_nil", func(t *testing.T) {
		if err := updatePolicyRulesOn(ctx, nodes[2], policyName, nil); err != nil {
			t.Fatalf("update rules (clear nil) on node 3: %v", err)
		}

		awaitConvergence(ctx, t, h)

		for i, c := range nodes {
			got := mustGetPolicy(ctx, t, c, policyName)
			if got.Rules != nil {
				t.Fatalf("node %d expected Rules == nil after nil-clear, got %s", i+1, formatRules(got.Rules))
			}
		}
	})

	t.Run("update_clear_empty_struct", func(t *testing.T) {
		if err := updatePolicyRulesOn(ctx, nodes[0], policyName, initialRules); err != nil {
			t.Fatalf("re-seed on node 1: %v", err)
		}

		awaitConvergence(ctx, t, h)

		if err := updatePolicyRulesOn(ctx, nodes[1], policyName, &client.PolicyRules{}); err != nil {
			t.Fatalf("clear empty on node 2: %v", err)
		}

		awaitConvergence(ctx, t, h)

		for i, c := range nodes {
			got := mustGetPolicy(ctx, t, c, policyName)
			if got.Rules != nil {
				t.Fatalf("node %d expected Rules == nil after empty-struct clear, got %s", i+1, formatRules(got.Rules))
			}
		}
	})

	t.Run("preserve_when_updating_other_field", func(t *testing.T) {
		if err := updatePolicyRulesOn(ctx, nodes[2], policyName, initialRules); err != nil {
			t.Fatalf("re-seed on node 3: %v", err)
		}

		awaitConvergence(ctx, t, h)

		writer := nodes[0]

		current := mustGetPolicy(ctx, t, writer, policyName)
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

		if err := writer.UpdatePolicy(ctx, policyName, opts); err != nil {
			t.Fatalf("update policy on node 1: %v", err)
		}

		awaitConvergence(ctx, t, h)

		for i, c := range nodes {
			got := mustGetPolicy(ctx, t, c, policyName)

			if got.Var5qi != 5 {
				t.Fatalf("node %d Var5qi: got %d, want 5", i+1, got.Var5qi)
			}

			if !rulesEqual(got.Rules, initialRules) {
				t.Fatalf("node %d rules dropped when updating Var5qi:\ngot  %s\nwant %s",
					i+1, formatRules(got.Rules), formatRules(initialRules))
			}
		}
	})

	negatives := []struct {
		name   string
		writer int
		rules  *client.PolicyRules
		want   string
	}{
		{
			name:   "bad_action",
			writer: 0,
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: "x", Protocol: 6, PortLow: 80, PortHigh: 80, Action: "reject"},
			}},
			want: "action must be 'allow' or 'deny'",
		},
		{
			name:   "bad_cidr",
			writer: 1,
			rules: func() *client.PolicyRules {
				p := "not-a-cidr"

				return &client.PolicyRules{Uplink: []client.PolicyRule{
					{Description: "x", RemotePrefix: &p, Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
				}}
			}(),
			want: "invalid CIDR",
		},
		{
			name:   "protocol_out_of_range",
			writer: 2,
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: "x", Protocol: 999, PortLow: 80, PortHigh: 80, Action: "allow"},
			}},
			want: "protocol must be between 0 and 255",
		},
		{
			name:   "port_low_gt_port_high",
			writer: 0,
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: "x", Protocol: 6, PortLow: 100, PortHigh: 50, Action: "allow"},
			}},
			want: "port_low must be <= port_high",
		},
		{
			name:   "description_oversized",
			writer: 1,
			rules: &client.PolicyRules{Uplink: []client.PolicyRule{
				{Description: strings.Repeat("x", 257), Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			}},
			want: "256 characters or fewer",
		},
		{
			name:   "too_many_per_direction",
			writer: 2,
			rules:  &client.PolicyRules{Uplink: makeNRules(13)},
			want:   "uplink rules exceed maximum",
		},
	}

	if err := updatePolicyRulesOn(ctx, nodes[0], policyName, nil); err != nil {
		t.Fatalf("setup negatives: clear rules: %v", err)
	}

	awaitConvergence(ctx, t, h)

	for _, n := range negatives {
		n := n
		t.Run("negative_"+n.name, func(t *testing.T) {
			err := updatePolicyRulesOn(ctx, nodes[n.writer], policyName, n.rules)
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

			for i, c := range nodes {
				got := mustGetPolicy(ctx, t, c, policyName)
				if got.Rules != nil {
					t.Fatalf("node %d rules mutated by rejected update %s: %s", i+1, n.name, formatRules(got.Rules))
				}
			}
		})
	}
}

// updatePolicyRulesOn mirrors updatePolicyRules but lets the caller pick
// which node issues the write, so the rotation visits both leader-direct
// and follower-proxy paths.
func updatePolicyRulesOn(ctx context.Context, c *client.Client, name string, rules *client.PolicyRules) error {
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
