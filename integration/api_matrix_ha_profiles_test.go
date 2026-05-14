package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runProfilesHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	name := apiMatrixName("ha-profile")

	listAllOn := func(c *client.Client) *client.ListProfilesResponse {
		resp, err := c.ListProfiles(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list profiles: %v", err)
		}

		return resp
	}

	contains := func(items []client.Profile, name string) bool {
		for _, p := range items {
			if p.Name == name {
				return true
			}
		}

		return false
	}

	baseline := listAllOn(h.Leader).TotalCount

	createOpts := &client.CreateProfileOptions{
		Name:           name,
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	if err := nodes[0].CreateProfile(ctx, createOpts); err != nil {
		t.Fatalf("create profile on node 1: %v", err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := h.Leader.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete profile: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetProfile(ctx, &client.GetProfileOptions{Name: name})
		if err != nil {
			t.Fatalf("node %d get after create: %v", i+1, err)
		}

		if got.Name != createOpts.Name ||
			got.UeAmbrUplink != createOpts.UeAmbrUplink ||
			got.UeAmbrDownlink != createOpts.UeAmbrDownlink {
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
		opts   client.UpdateProfileOptions
		assert func(t *testing.T, p *client.Profile)
	}{
		{
			field:  "UeAmbrUplink",
			writer: 1,
			opts:   client.UpdateProfileOptions{UeAmbrUplink: "500 Mbps", UeAmbrDownlink: createOpts.UeAmbrDownlink},
			assert: func(t *testing.T, p *client.Profile) {
				if p.UeAmbrUplink != "500 Mbps" {
					t.Fatalf("UeAmbrUplink: got %q, want %q", p.UeAmbrUplink, "500 Mbps")
				}
			},
		},
		{
			field:  "UeAmbrDownlink",
			writer: 2,
			opts:   client.UpdateProfileOptions{UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "1 Gbps"},
			assert: func(t *testing.T, p *client.Profile) {
				if p.UeAmbrDownlink != "1 Gbps" {
					t.Fatalf("UeAmbrDownlink: got %q, want %q", p.UeAmbrDownlink, "1 Gbps")
				}
			},
		},
	}

	for _, tc := range updateCases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateProfile(ctx, name, &tc.opts); err != nil {
				t.Fatalf("update on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				got, err := c.GetProfile(ctx, &client.GetProfileOptions{Name: name})
				if err != nil {
					t.Fatalf("node %d get after update: %v", i+1, err)
				}

				tc.assert(t, got)
			}
		})
	}

	if err := nodes[2].DeleteProfile(ctx, &client.DeleteProfileOptions{Name: name}); err != nil {
		t.Fatalf("delete profile on node 3: %v", err)
	}

	deleted = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		_, err := c.GetProfile(ctx, &client.GetProfileOptions{Name: name})
		assertNotFound(t, err, fmt.Sprintf("profile on node %d after delete", i+1))

		list := listAllOn(c)
		if list.TotalCount != baseline {
			t.Fatalf("node %d count after delete: got %d, want %d", i+1, list.TotalCount, baseline)
		}

		if contains(list.Items, name) {
			t.Fatalf("node %d list after delete still contains %q", i+1, name)
		}
	}
}
