package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runSlicesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	name := apiMatrixName("slice")

	listAll := func() *client.ListSlicesResponse {
		resp, err := c.ListSlices(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list slices: %v", err)
		}

		return resp
	}

	contains := func(items []client.Slice, name string) bool {
		for _, s := range items {
			if s.Name == name {
				return true
			}
		}

		return false
	}

	baseline := listAll()

	createOpts := &client.CreateSliceOptions{
		Name: name,
		Sst:  1,
		Sd:   "010203",
	}

	if err := c.CreateSlice(ctx, createOpts); err != nil {
		t.Fatalf("create slice %q: %v", name, err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete slice %q: %v", name, err)
		}
	})

	got, err := c.GetSlice(ctx, &client.GetSliceOptions{Name: name})
	if err != nil {
		t.Fatalf("get slice %q after create: %v", name, err)
	}

	if got.Name != createOpts.Name || got.Sst != createOpts.Sst || got.Sd != createOpts.Sd {
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
		opts   client.UpdateSliceOptions
		assert func(t *testing.T, s *client.Slice)
	}{
		{
			field: "Sst",
			opts:  client.UpdateSliceOptions{Sst: 2, Sd: createOpts.Sd},
			assert: func(t *testing.T, s *client.Slice) {
				if s.Sst != 2 {
					t.Fatalf("Sst: got %d, want 2", s.Sst)
				}
			},
		},
		{
			field: "Sd",
			opts:  client.UpdateSliceOptions{Sst: 2, Sd: "abcdef"},
			assert: func(t *testing.T, s *client.Slice) {
				if s.Sd != "abcdef" {
					t.Fatalf("Sd: got %q, want %q", s.Sd, "abcdef")
				}
			},
		},
	}

	for _, tc := range updateCases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := c.UpdateSlice(ctx, name, &tc.opts); err != nil {
				t.Fatalf("update slice %q (%s): %v", name, tc.field, err)
			}

			updated, err := c.GetSlice(ctx, &client.GetSliceOptions{Name: name})
			if err != nil {
				t.Fatalf("get slice %q after update: %v", name, err)
			}

			tc.assert(t, updated)
		})
	}

	if err := c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: name}); err != nil {
		t.Fatalf("delete slice %q: %v", name, err)
	}

	deleted = true

	_, err = c.GetSlice(ctx, &client.GetSliceOptions{Name: name})
	assertNotFound(t, err, "slice after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if contains(afterDelete.Items, name) {
		t.Fatalf("list after delete still contains %q", name)
	}
}
