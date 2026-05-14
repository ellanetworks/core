package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runPoliciesMatrix stands up a primary and a secondary of each
// referenced resource (Profile, Slice, Data Network) so it can round-trip
// the three reference fields on Update.
//
// Each Update sub-case starts from the current Get state because a
// zero-valued field in the Update body silently overwrites live state on
// the server.
func runPoliciesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	profileA := apiMatrixName("policy-profile-a")
	profileB := apiMatrixName("policy-profile-b")
	sliceA := apiMatrixName("policy-slice-a")
	sliceB := apiMatrixName("policy-slice-b")
	dnA := apiMatrixName("policy-dn-a")
	dnB := apiMatrixName("policy-dn-b")
	name := apiMatrixName("policy")

	for _, p := range []string{profileA, profileB} {
		p := p

		if err := c.CreateProfile(ctx, &client.CreateProfileOptions{
			Name:           p,
			UeAmbrUplink:   "100 Mbps",
			UeAmbrDownlink: "100 Mbps",
		}); err != nil {
			t.Fatalf("create dep profile %q: %v", p, err)
		}

		t.Cleanup(func() {
			if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: p}); err != nil {
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

		if err := c.CreateSlice(ctx, &client.CreateSliceOptions{Name: s.name, Sst: s.sst, Sd: s.sd}); err != nil {
			t.Fatalf("create dep slice %q: %v", s.name, err)
		}

		t.Cleanup(func() {
			if err := c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: s.name}); err != nil {
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

		if err := c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
			Name:     d.name,
			IPv4Pool: d.pool,
			DNS:      "8.8.8.8",
			Mtu:      1500,
		}); err != nil {
			t.Fatalf("create dep data network %q: %v", d.name, err)
		}

		t.Cleanup(func() {
			if err := c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: d.name}); err != nil {
				t.Logf("cleanup: delete dep data network %q: %v", d.name, err)
			}
		})
	}

	listAll := func() *client.ListPoliciesResponse {
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

	baseline := listAll()

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

	if err := c.CreatePolicy(ctx, createOpts); err != nil {
		t.Fatalf("create policy %q: %v", name, err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete policy %q: %v", name, err)
		}
	})

	got, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
	if err != nil {
		t.Fatalf("get policy %q after create: %v", name, err)
	}

	if got.Name != createOpts.Name ||
		got.ProfileName != createOpts.ProfileName ||
		got.SliceName != createOpts.SliceName ||
		got.DataNetworkName != createOpts.DataNetworkName ||
		got.SessionAmbrUplink != createOpts.SessionAmbrUplink ||
		got.SessionAmbrDownlink != createOpts.SessionAmbrDownlink ||
		got.Var5qi != createOpts.Var5qi ||
		got.Arp != createOpts.Arp {
		t.Fatalf("post-create round-trip mismatch: got %+v, want %+v", got, createOpts)
	}

	afterCreate := listAll()
	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if !contains(afterCreate.Items, name) {
		t.Fatalf("list after create missing %q", name)
	}

	updateCases := []struct {
		field  string
		mutate func(o *client.UpdatePolicyOptions)
		assert func(t *testing.T, p *client.Policy)
	}{
		{
			field: "ProfileName",
			mutate: func(o *client.UpdatePolicyOptions) {
				o.ProfileName = profileB
			},
			assert: func(t *testing.T, p *client.Policy) {
				if p.ProfileName != profileB {
					t.Fatalf("ProfileName: got %q, want %q", p.ProfileName, profileB)
				}
			},
		},
		{
			field: "SliceName",
			mutate: func(o *client.UpdatePolicyOptions) {
				o.SliceName = sliceB
			},
			assert: func(t *testing.T, p *client.Policy) {
				if p.SliceName != sliceB {
					t.Fatalf("SliceName: got %q, want %q", p.SliceName, sliceB)
				}
			},
		},
		{
			field: "DataNetworkName",
			mutate: func(o *client.UpdatePolicyOptions) {
				o.DataNetworkName = dnB
			},
			assert: func(t *testing.T, p *client.Policy) {
				if p.DataNetworkName != dnB {
					t.Fatalf("DataNetworkName: got %q, want %q", p.DataNetworkName, dnB)
				}
			},
		},
		{
			field: "SessionAmbrUplink",
			mutate: func(o *client.UpdatePolicyOptions) {
				o.SessionAmbrUplink = "200 Mbps"
			},
			assert: func(t *testing.T, p *client.Policy) {
				if p.SessionAmbrUplink != "200 Mbps" {
					t.Fatalf("SessionAmbrUplink: got %q, want %q", p.SessionAmbrUplink, "200 Mbps")
				}
			},
		},
		{
			field: "SessionAmbrDownlink",
			mutate: func(o *client.UpdatePolicyOptions) {
				o.SessionAmbrDownlink = "400 Mbps"
			},
			assert: func(t *testing.T, p *client.Policy) {
				if p.SessionAmbrDownlink != "400 Mbps" {
					t.Fatalf("SessionAmbrDownlink: got %q, want %q", p.SessionAmbrDownlink, "400 Mbps")
				}
			},
		},
		{
			field: "Var5qi",
			mutate: func(o *client.UpdatePolicyOptions) {
				o.Var5qi = 8
			},
			assert: func(t *testing.T, p *client.Policy) {
				if p.Var5qi != 8 {
					t.Fatalf("Var5qi: got %d, want 8", p.Var5qi)
				}
			},
		},
		{
			field: "Arp",
			mutate: func(o *client.UpdatePolicyOptions) {
				o.Arp = 5
			},
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
			current, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
			if err != nil {
				t.Fatalf("get policy %q before update: %v", name, err)
			}

			opts := updateOptsFromPolicy(current)
			tc.mutate(&opts)

			if err := c.UpdatePolicy(ctx, name, &opts); err != nil {
				t.Fatalf("update policy %q (%s): %v", name, tc.field, err)
			}

			updated, err := c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
			if err != nil {
				t.Fatalf("get policy %q after update: %v", name, err)
			}

			tc.assert(t, updated)
		})
	}

	if err := c.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: name}); err != nil {
		t.Fatalf("delete policy %q: %v", name, err)
	}

	deleted = true

	_, err = c.GetPolicy(ctx, &client.GetPolicyOptions{Name: name})
	assertNotFound(t, err, "policy after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if contains(afterDelete.Items, name) {
		t.Fatalf("list after delete still contains %q", name)
	}
}

// updateOptsFromPolicy mirrors a Get response into Update options so
// per-field sub-cases can mutate exactly one field.
func updateOptsFromPolicy(p *client.Policy) client.UpdatePolicyOptions {
	return client.UpdatePolicyOptions{
		ProfileName:         p.ProfileName,
		SliceName:           p.SliceName,
		DataNetworkName:     p.DataNetworkName,
		SessionAmbrUplink:   p.SessionAmbrUplink,
		SessionAmbrDownlink: p.SessionAmbrDownlink,
		Var5qi:              p.Var5qi,
		Arp:                 p.Arp,
		Rules:               p.Rules,
	}
}
