package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runPoliciesMatrix exercises CRUD for policies. Policies reference a
// Profile, a Slice, and a Data Network, so the runner sets all three up
// (with t.Cleanup teardown) before running the matrix. See
// api_matrix_profiles_test.go for the matrix shape.
func runPoliciesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	profileName := apiMatrixName("policy-profile")
	sliceName := apiMatrixName("policy-slice")
	dnName := apiMatrixName("policy-dn")
	name := apiMatrixName("policy")

	if err := c.CreateProfile(ctx, &client.CreateProfileOptions{
		Name:           profileName,
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "100 Mbps",
	}); err != nil {
		t.Fatalf("create dep profile: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profileName}); err != nil {
			t.Logf("cleanup: delete dep profile: %v", err)
		}
	})

	if err := c.CreateSlice(ctx, &client.CreateSliceOptions{
		Name: sliceName,
		Sst:  1,
		Sd:   "010203",
	}); err != nil {
		t.Fatalf("create dep slice: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: sliceName}); err != nil {
			t.Logf("cleanup: delete dep slice: %v", err)
		}
	})

	if err := c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name:     dnName,
		IPv4Pool: "10.252.0.0/16",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}); err != nil {
		t.Fatalf("create dep data network: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: dnName}); err != nil {
			t.Logf("cleanup: delete dep data network: %v", err)
		}
	})

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
		ProfileName:         profileName,
		SliceName:           sliceName,
		DataNetworkName:     dnName,
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

	base := client.UpdatePolicyOptions{
		ProfileName:         createOpts.ProfileName,
		SliceName:           createOpts.SliceName,
		DataNetworkName:     createOpts.DataNetworkName,
		SessionAmbrUplink:   createOpts.SessionAmbrUplink,
		SessionAmbrDownlink: createOpts.SessionAmbrDownlink,
		Var5qi:              createOpts.Var5qi,
		Arp:                 createOpts.Arp,
	}

	updateCases := []struct {
		field  string
		mutate func(o *client.UpdatePolicyOptions)
		assert func(t *testing.T, p *client.Policy)
	}{
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
			opts := base
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
