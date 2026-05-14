package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runRoutesHAMatrix exercises the routes API across the cluster and
// asserts the local-only invariant: a route created on one node is
// visible only on that node, never the other two.
//
// The gateway is another node's n6 address so the kernel accepts the
// route (gateway must be on the writer's directly-connected n6 subnet).
func runRoutesHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	if DetectIPFamily() != IPv4Only {
		t.Skipf("HA routes matrix runs IPv4 only; got %s", DetectIPFamily())
	}

	const writerIdx = 0

	writer := h.Clients[writerIdx]

	listAllOn := func(c *client.Client) *client.ListRoutesResponse {
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

	baselines := make(map[int]*client.ListRoutesResponse, len(h.Clients))
	for i, c := range h.Clients {
		baselines[i] = listAllOn(c)
	}

	createOpts := &client.CreateRouteOptions{
		Destination: "192.0.2.0/24",
		Gateway:     "10.6.0.12",
		Interface:   "n6",
		Metric:      100,
	}

	if err := writer.CreateRoute(ctx, createOpts); err != nil {
		t.Fatalf("create route on node %d: %v", writerIdx+1, err)
	}

	writerList := listAllOn(writer)

	created := findByDestination(writerList.Items, createOpts.Destination)
	if created == nil {
		t.Fatalf("writer node %d missing destination %q after create", writerIdx+1, createOpts.Destination)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := writer.DeleteRoute(ctx, &client.DeleteRouteOptions{ID: created.ID}); err != nil {
			t.Logf("cleanup: delete route %d: %v", created.ID, err)
		}
	})

	if writerList.TotalCount != baselines[writerIdx].TotalCount+1 {
		t.Fatalf("writer node %d count after create: got %d, want %d",
			writerIdx+1, writerList.TotalCount, baselines[writerIdx].TotalCount+1)
	}

	if created.Gateway != createOpts.Gateway ||
		created.Interface != createOpts.Interface ||
		created.Metric != createOpts.Metric {
		t.Fatalf("writer node %d post-create mismatch: got %+v, want %+v", writerIdx+1, created, createOpts)
	}

	got, err := writer.GetRoute(ctx, &client.GetRouteOptions{ID: created.ID})
	if err != nil {
		t.Fatalf("get route %d on writer: %v", created.ID, err)
	}

	if got.ID != created.ID || got.Destination != createOpts.Destination {
		t.Fatalf("get route mismatch: got %+v, want id=%d destination=%q", got, created.ID, createOpts.Destination)
	}

	stabilizeLocal()

	for i, c := range h.Clients {
		if i == writerIdx {
			continue
		}

		other := listAllOn(c)
		if other.TotalCount != baselines[i].TotalCount {
			t.Fatalf("node %d count after writer create: got %d, want %d (route leaked from node %d)",
				i+1, other.TotalCount, baselines[i].TotalCount, writerIdx+1)
		}

		if findByDestination(other.Items, createOpts.Destination) != nil {
			t.Fatalf("node %d list contains destination %q after writer-only create (locality breach)",
				i+1, createOpts.Destination)
		}

		if _, err := c.GetRoute(ctx, &client.GetRouteOptions{ID: created.ID}); err == nil {
			t.Fatalf("node %d returned route id=%d that was created only on node %d",
				i+1, created.ID, writerIdx+1)
		}
	}

	if err := writer.DeleteRoute(ctx, &client.DeleteRouteOptions{ID: created.ID}); err != nil {
		t.Fatalf("delete route %d on writer: %v", created.ID, err)
	}

	deleted = true

	_, err = writer.GetRoute(ctx, &client.GetRouteOptions{ID: created.ID})
	assertNotFound(t, err, "route on writer after delete")

	writerFinal := listAllOn(writer)
	if writerFinal.TotalCount != baselines[writerIdx].TotalCount {
		t.Fatalf("writer node %d count after delete: got %d, want %d",
			writerIdx+1, writerFinal.TotalCount, baselines[writerIdx].TotalCount)
	}

	if findByDestination(writerFinal.Items, createOpts.Destination) != nil {
		t.Fatalf("writer node %d list still contains destination %q after delete",
			writerIdx+1, createOpts.Destination)
	}
}
