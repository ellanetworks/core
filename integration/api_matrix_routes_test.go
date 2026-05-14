package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runRoutesMatrix exercises CRD for routes. The Routes API has no Update
// verb (server.go:153-156), so this is a 4-step matrix:
//
//	List → Create → Get/List → Delete → Get(404)/List
//
// The bootstrap installs a default route to 8.8.8.8 (tester_env_test.go),
// so we use a different destination here to avoid collisions.
func runRoutesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	listAll := func() *client.ListRoutesResponse {
		resp, err := c.ListRoutes(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list routes: %v", err)
		}

		return resp
	}

	findByDestination := func(items []client.Route, dest string) *client.Route {
		for i := range items {
			if items[i].Destination == dest {
				return &items[i]
			}
		}

		return nil
	}

	baseline := listAll()

	createOpts := &client.CreateRouteOptions{
		Destination: "192.0.2.0/24",
		Gateway:     N6RouterIPv4Address(),
		Interface:   "n6",
		Metric:      100,
	}

	if err := c.CreateRoute(ctx, createOpts); err != nil {
		t.Fatalf("create route: %v", err)
	}

	afterCreate := listAll()

	created := findByDestination(afterCreate.Items, createOpts.Destination)
	if created == nil {
		t.Fatalf("list after create missing destination %q", createOpts.Destination)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteRoute(ctx, &client.DeleteRouteOptions{ID: created.ID}); err != nil {
			t.Logf("cleanup: delete route %d: %v", created.ID, err)
		}
	})

	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if created.Gateway != createOpts.Gateway ||
		created.Interface != createOpts.Interface ||
		created.Metric != createOpts.Metric {
		t.Fatalf("post-create round-trip mismatch: got %+v, want %+v", created, createOpts)
	}

	got, err := c.GetRoute(ctx, &client.GetRouteOptions{ID: created.ID})
	if err != nil {
		t.Fatalf("get route %d: %v", created.ID, err)
	}

	if got.ID != created.ID || got.Destination != createOpts.Destination {
		t.Fatalf("get route mismatch: got %+v, want id=%d destination=%q", got, created.ID, createOpts.Destination)
	}

	if err := c.DeleteRoute(ctx, &client.DeleteRouteOptions{ID: created.ID}); err != nil {
		t.Fatalf("delete route %d: %v", created.ID, err)
	}

	deleted = true

	_, err = c.GetRoute(ctx, &client.GetRouteOptions{ID: created.ID})
	assertNotFound(t, err, "route after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if findByDestination(afterDelete.Items, createOpts.Destination) != nil {
		t.Fatalf("list after delete still contains destination %q", createOpts.Destination)
	}
}
