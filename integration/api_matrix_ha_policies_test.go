package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runPoliciesHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	leader := h.Leader

	profileA := apiMatrixName("ha-policy-profile-a")
	profileB := apiMatrixName("ha-policy-profile-b")
	sliceA := apiMatrixName("ha-policy-slice-a")
	sliceB := apiMatrixName("ha-policy-slice-b")
	dnA := apiMatrixName("ha-policy-dn-a")
	dnB := apiMatrixName("ha-policy-dn-b")
	name := apiMatrixName("ha-policy")

	for _, p := range []string{profileA, profileB} {
		p := p

		if err := leader.CreateProfile(ctx, &client.CreateProfileOptions{
			Name:           p,
			UeAmbrUplink:   "100 Mbps",
			UeAmbrDownlink: "100 Mbps",
		}); err != nil {
			t.Fatalf("create dep profile %q: %v", p, err)
		}

		t.Cleanup(func() {
			if err := leader.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: p}); err != nil {
				t.Logf("cleanup: delete dep profile %q: %v", p, err)
			}
		})
	}

	for _, s := range []struct {
		name string
		sst  int
		sd   string
	}{
		{sliceA, 1, "010203"},
		{sliceB, 2, "040506"},
	} {
		s := s

		if err := leader.CreateSlice(ctx, &client.CreateSliceOptions{Name: s.name, Sst: s.sst, Sd: s.sd}); err != nil {
			t.Fatalf("create dep slice %q: %v", s.name, err)
		}

		t.Cleanup(func() {
			if err := leader.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: s.name}); err != nil {
				t.Logf("cleanup: delete dep slice %q: %v", s.name, err)
			}
		})
	}

	for _, d := range []struct {
		name string
		pool string
	}{
		{dnA, "10.252.0.0/16"},
		{dnB, "10.249.0.0/16"},
	} {
		d := d

		if err := leader.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
			Name:     d.name,
			IPv4Pool: d.pool,
			DNS:      "8.8.8.8",
			Mtu:      1500,
		}); err != nil {
			t.Fatalf("create dep data network %q: %v", d.name, err)
		}

		t.Cleanup(func() {
			if err := leader.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: d.name}); err != nil {
				t.Logf("cleanup: delete dep data network %q: %v", d.name, err)
			}
		})
	}

	awaitConvergence(ctx, t, h)

	listAllOn := func(c *client.Client) *client.ListPoliciesResponse {
		resp, err := c.ListPolicies(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list policies: %v", err)
		}

		return resp
	}

	contains := func(items []client.Policy, name string) bool {
		for _, p := range items {
			if p.Name == name {
				return true
			}
		}

		return false
	}

	baseline := listAllOn(leader).TotalCount

	createOpts := &client.CreatePolicyOptions{
		Name:                name,
		ProfileName:         profileA,
		SliceName:           sliceA,
		DataNetworkName:     dnA,
		SessionAmbrUplink:   "50 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 8,
	}

	if err := nodes[0].CreatePolicy(ctx, createOpts); err != nil {
		t.Fatalf("create policy on node 1: %v", err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := leader.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete policy: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
		if err != nil {
			t.Fatalf("node %d get after create: %v", i+1, err)
		}

		if got.Name != createOpts.Name ||
			got.ProfileName != createOpts.ProfileName ||
			got.SliceName != createOpts.SliceName ||
			got.DataNetworkName != createOpts.DataNetworkName ||
			got.SessionAmbrUplink != createOpts.SessionAmbrUplink ||
			got.SessionAmbrDownlink != createOpts.SessionAmbrDownlink ||
			got.Var5qi != createOpts.Var5qi ||
			got.Arp != createOpts.Arp {
			t.Fatalf("node %d post-create mismatch: got %+v, want %+v", i+1, got, createOpts)
		}

		list := listAllOn(c)
		if list.TotalCount != baseline+1 {
			t.Fatalf("node %d count after create: got %d, want %d", i+1, list.TotalCount, baseline+1)
		}

		if !contains(list.Items, name) {
			t.Fatalf("node %d list after create missing %q", i+1, name)
		}
	}

	updateCases := []struct {
		field  string
		writer int
		mutate func(o *client.UpdatePolicyOptions)
		assert func(t *testing.T, p *client.Policy)
	}{
		{
			field:  "ProfileName",
			writer: 1,
			mutate: func(o *client.UpdatePolicyOptions) { o.ProfileName = profileB },
			assert: func(t *testing.T, p *client.Policy) {
				if p.ProfileName != profileB {
					t.Fatalf("ProfileName: got %q, want %q", p.ProfileName, profileB)
				}
			},
		},
		{
			field:  "SliceName",
			writer: 2,
			mutate: func(o *client.UpdatePolicyOptions) { o.SliceName = sliceB },
			assert: func(t *testing.T, p *client.Policy) {
				if p.SliceName != sliceB {
					t.Fatalf("SliceName: got %q, want %q", p.SliceName, sliceB)
				}
			},
		},
		{
			field:  "DataNetworkName",
			writer: 0,
			mutate: func(o *client.UpdatePolicyOptions) { o.DataNetworkName = dnB },
			assert: func(t *testing.T, p *client.Policy) {
				if p.DataNetworkName != dnB {
					t.Fatalf("DataNetworkName: got %q, want %q", p.DataNetworkName, dnB)
				}
			},
		},
		{
			field:  "SessionAmbrUplink",
			writer: 1,
			mutate: func(o *client.UpdatePolicyOptions) { o.SessionAmbrUplink = "200 Mbps" },
			assert: func(t *testing.T, p *client.Policy) {
				if p.SessionAmbrUplink != "200 Mbps" {
					t.Fatalf("SessionAmbrUplink: got %q, want %q", p.SessionAmbrUplink, "200 Mbps")
				}
			},
		},
		{
			field:  "SessionAmbrDownlink",
			writer: 2,
			mutate: func(o *client.UpdatePolicyOptions) { o.SessionAmbrDownlink = "400 Mbps" },
			assert: func(t *testing.T, p *client.Policy) {
				if p.SessionAmbrDownlink != "400 Mbps" {
					t.Fatalf("SessionAmbrDownlink: got %q, want %q", p.SessionAmbrDownlink, "400 Mbps")
				}
			},
		},
		{
			field:  "Var5qi",
			writer: 0,
			mutate: func(o *client.UpdatePolicyOptions) { o.Var5qi = 8 },
			assert: func(t *testing.T, p *client.Policy) {
				if p.Var5qi != 8 {
					t.Fatalf("Var5qi: got %d, want 8", p.Var5qi)
				}
			},
		},
		{
			field:  "Arp",
			writer: 1,
			mutate: func(o *client.UpdatePolicyOptions) { o.Arp = 5 },
			assert: func(t *testing.T, p *client.Policy) {
				if p.Arp != 5 {
					t.Fatalf("Arp: got %d, want 5", p.Arp)
				}
			},
		},
	}

	for _, tc := range updateCases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			writer := nodes[tc.writer]

			current, err := writer.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
			if err != nil {
				t.Fatalf("get policy on node %d before update: %v", tc.writer+1, err)
			}

			opts := updateOptsFromPolicy(current)
			tc.mutate(&opts)

			if err := writer.UpdatePolicy(ctx, name, &opts); err != nil {
				t.Fatalf("update on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				got, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
				if err != nil {
					t.Fatalf("node %d get after update: %v", i+1, err)
				}

				tc.assert(t, got)
			}
		})
	}

	if err := nodes[2].DeletePolicy(ctx, &client.DeletePolicyOptions{Name: name}); err != nil {
		t.Fatalf("delete policy on node 3: %v", err)
	}

	deleted = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		_, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
		assertNotFound(t, err, fmt.Sprintf("policy on node %d after delete", i+1))

		list := listAllOn(c)
		if list.TotalCount != baseline {
			t.Fatalf("node %d count after delete: got %d, want %d", i+1, list.TotalCount, baseline)
		}

		if contains(list.Items, name) {
			t.Fatalf("node %d list after delete still contains %q", i+1, name)
		}
	}
}
