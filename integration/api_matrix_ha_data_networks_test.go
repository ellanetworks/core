package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runDataNetworksHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	name := apiMatrixName("ha-dn")

	listAllOn := func(c *client.Client) *client.ListDataNetworksResponse {
		resp, err := c.ListDataNetworks(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list data networks: %v", err)
		}

		return resp
	}

	contains := func(items []client.DataNetwork, name string) bool {
		for _, dn := range items {
			if dn.Name == name {
				return true
			}
		}

		return false
	}

	baseline := listAllOn(h.Leader).TotalCount

	createOpts := &client.CreateDataNetworkOptions{
		Name:     name,
		IPv4Pool: "10.250.0.0/16",
		IPv6Pool: "fd99::/48",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}

	if err := nodes[0].CreateDataNetwork(ctx, createOpts); err != nil {
		t.Fatalf("create data network on node 1: %v", err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := h.Leader.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete data network: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: name})
		if err != nil {
			t.Fatalf("node %d get after create: %v", i+1, err)
		}

		if got.Name != createOpts.Name ||
			got.IPv4Pool != createOpts.IPv4Pool ||
			got.IPv6Pool != createOpts.IPv6Pool ||
			got.DNS != createOpts.DNS ||
			got.Mtu != createOpts.Mtu {
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

	base := *createOpts

	updateCases := []struct {
		field  string
		writer int
		mutate func(o *client.UpdateDataNetworkOptions)
		assert func(t *testing.T, dn *client.DataNetwork)
	}{
		{
			field:  "IPv4Pool",
			writer: 1,
			mutate: func(o *client.UpdateDataNetworkOptions) { o.IPv4Pool = "10.251.0.0/16" },
			assert: func(t *testing.T, dn *client.DataNetwork) {
				if dn.IPv4Pool != "10.251.0.0/16" {
					t.Fatalf("IPv4Pool: got %q, want %q", dn.IPv4Pool, "10.251.0.0/16")
				}
			},
		},
		{
			field:  "IPv6Pool",
			writer: 2,
			mutate: func(o *client.UpdateDataNetworkOptions) { o.IPv6Pool = "fd9a::/48" },
			assert: func(t *testing.T, dn *client.DataNetwork) {
				if dn.IPv6Pool != "fd9a::/48" {
					t.Fatalf("IPv6Pool: got %q, want %q", dn.IPv6Pool, "fd9a::/48")
				}
			},
		},
		{
			field:  "DNS",
			writer: 0,
			mutate: func(o *client.UpdateDataNetworkOptions) { o.DNS = "1.1.1.1" },
			assert: func(t *testing.T, dn *client.DataNetwork) {
				if dn.DNS != "1.1.1.1" {
					t.Fatalf("DNS: got %q, want %q", dn.DNS, "1.1.1.1")
				}
			},
		},
		{
			field:  "Mtu",
			writer: 1,
			mutate: func(o *client.UpdateDataNetworkOptions) { o.Mtu = 1400 },
			assert: func(t *testing.T, dn *client.DataNetwork) {
				if dn.Mtu != 1400 {
					t.Fatalf("Mtu: got %d, want %d", dn.Mtu, 1400)
				}
			},
		},
	}

	for _, tc := range updateCases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			opts := client.UpdateDataNetworkOptions{
				Name:     name,
				IPv4Pool: base.IPv4Pool,
				IPv6Pool: base.IPv6Pool,
				DNS:      base.DNS,
				Mtu:      base.Mtu,
			}

			tc.mutate(&opts)

			if err := nodes[tc.writer].UpdateDataNetwork(ctx, &opts); err != nil {
				t.Fatalf("update on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				got, err := c.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: name})
				if err != nil {
					t.Fatalf("node %d get after update: %v", i+1, err)
				}

				tc.assert(t, got)
			}
		})
	}

	if err := nodes[2].DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: name}); err != nil {
		t.Fatalf("delete data network on node 3: %v", err)
	}

	deleted = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		_, err := c.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: name})
		assertNotFound(t, err, fmt.Sprintf("data network on node %d after delete", i+1))

		list := listAllOn(c)
		if list.TotalCount != baseline {
			t.Fatalf("node %d count after delete: got %d, want %d", i+1, list.TotalCount, baseline)
		}

		if contains(list.Items, name) {
			t.Fatalf("node %d list after delete still contains %q", i+1, name)
		}
	}
}
