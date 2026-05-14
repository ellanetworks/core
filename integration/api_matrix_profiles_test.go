package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runProfilesMatrix exercises every CRUD verb for the Profiles resource and
// every settable field on Update. The matrix shape is:
//
//  1. List → snapshot baseline count
//  2. Create
//  3. Get → assert all create fields round-trip
//  4. List → assert count == baseline+1 and entity present
//  5. Update each settable field via a sub-table; Get after each mutation
//  6. Delete
//  7. Get → assert 404
//  8. List → assert count == baseline
func runProfilesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	name := apiMatrixName("profile")

	listAll := func() *client.ListProfilesResponse {
		resp, err := c.ListProfiles(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list profiles: %v", err)
		}

		return resp
	}

	containsProfile := func(items []client.Profile, name string) bool {
		for _, p := range items {
			if p.Name == name {
				return true
			}
		}

		return false
	}

	baseline := listAll()

	createOpts := &client.CreateProfileOptions{
		Name:           name,
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	if err := c.CreateProfile(ctx, createOpts); err != nil {
		t.Fatalf("create profile %q: %v", name, err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete profile %q: %v", name, err)
		}
	})

	got, err := c.GetProfile(ctx, &client.GetProfileOptions{Name: name})
	if err != nil {
		t.Fatalf("get profile %q after create: %v", name, err)
	}

	if got.Name != createOpts.Name ||
		got.UeAmbrUplink != createOpts.UeAmbrUplink ||
		got.UeAmbrDownlink != createOpts.UeAmbrDownlink {
		t.Fatalf("post-create round-trip mismatch: got %+v, want %+v", got, createOpts)
	}

	afterCreate := listAll()
	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if !containsProfile(afterCreate.Items, name) {
		t.Fatalf("list after create missing %q", name)
	}

	updateCases := []struct {
		field  string
		opts   client.UpdateProfileOptions
		assert func(t *testing.T, p *client.Profile)
	}{
		{
			field: "UeAmbrUplink",
			opts:  client.UpdateProfileOptions{UeAmbrUplink: "500 Mbps", UeAmbrDownlink: createOpts.UeAmbrDownlink},
			assert: func(t *testing.T, p *client.Profile) {
				if p.UeAmbrUplink != "500 Mbps" {
					t.Fatalf("UeAmbrUplink: got %q, want %q", p.UeAmbrUplink, "500 Mbps")
				}
			},
		},
		{
			field: "UeAmbrDownlink",
			opts:  client.UpdateProfileOptions{UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "1 Gbps"},
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
			if err := c.UpdateProfile(ctx, name, &tc.opts); err != nil {
				t.Fatalf("update profile %q (%s): %v", name, tc.field, err)
			}

			updated, err := c.GetProfile(ctx, &client.GetProfileOptions{Name: name})
			if err != nil {
				t.Fatalf("get profile %q after update: %v", name, err)
			}

			tc.assert(t, updated)
		})
	}

	if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: name}); err != nil {
		t.Fatalf("delete profile %q: %v", name, err)
	}

	deleted = true

	_, err = c.GetProfile(ctx, &client.GetProfileOptions{Name: name})
	assertNotFound(t, err, "profile after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if containsProfile(afterDelete.Items, name) {
		t.Fatalf("list after delete still contains %q", name)
	}
}
