package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runSlicesHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	name := apiMatrixName("ha-slice")

	listAllOn := func(c *client.Client) *client.ListSlicesResponse {
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

	baseline := listAllOn(h.Leader).TotalCount

	createOpts := &client.CreateSliceOptions{Name: name, Sst: 1, Sd: "010203"}

	if err := nodes[0].CreateSlice(ctx, createOpts); err != nil {
		t.Fatalf("create slice on node 1: %v", err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := h.Leader.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete slice: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetSlice(ctx, &client.GetSliceOptions{Name: name})
		if err != nil {
			t.Fatalf("node %d get after create: %v", i+1, err)
		}

		if got.Name != createOpts.Name || got.Sst != createOpts.Sst || got.Sd != createOpts.Sd {
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
		opts   client.UpdateSliceOptions
		assert func(t *testing.T, s *client.Slice)
	}{
		{
			field:  "Sst",
			writer: 1,
			opts:   client.UpdateSliceOptions{Sst: 2, Sd: createOpts.Sd},
			assert: func(t *testing.T, s *client.Slice) {
				if s.Sst != 2 {
					t.Fatalf("Sst: got %d, want 2", s.Sst)
				}
			},
		},
		{
			field:  "Sd",
			writer: 2,
			opts:   client.UpdateSliceOptions{Sst: 2, Sd: "abcdef"},
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
			if err := nodes[tc.writer].UpdateSlice(ctx, name, &tc.opts); err != nil {
				t.Fatalf("update on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				got, err := c.GetSlice(ctx, &client.GetSliceOptions{Name: name})
				if err != nil {
					t.Fatalf("node %d get after update: %v", i+1, err)
				}

				tc.assert(t, got)
			}
		})
	}

	if err := nodes[2].DeleteSlice(ctx, &client.DeleteSliceOptions{Name: name}); err != nil {
		t.Fatalf("delete slice on node 3: %v", err)
	}

	deleted = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		_, err := c.GetSlice(ctx, &client.GetSliceOptions{Name: name})
		assertNotFound(t, err, fmt.Sprintf("slice on node %d after delete", i+1))

		list := listAllOn(c)
		if list.TotalCount != baseline {
			t.Fatalf("node %d count after delete: got %d, want %d", i+1, list.TotalCount, baseline)
		}

		if contains(list.Items, name) {
			t.Fatalf("node %d list after delete still contains %q", i+1, name)
		}
	}
}
