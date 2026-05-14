package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runDataNetworksMatrix exercises CRUD for data networks. See
// api_matrix_profiles_test.go for the matrix shape.
//
// IPv4Pool, IPv6Pool, DNS, and Mtu are all round-tripped via Update.
func runDataNetworksMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	name := apiMatrixName("dn")

	listAll := func() *client.ListDataNetworksResponse {
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

	baseline := listAll()

	createOpts := &client.CreateDataNetworkOptions{
		Name:     name,
		IPv4Pool: "10.250.0.0/16",
		IPv6Pool: "fd99::/48",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}

	if err := c.CreateDataNetwork(ctx, createOpts); err != nil {
		t.Fatalf("create data network %q: %v", name, err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete data network %q: %v", name, err)
		}
	})

	got, err := c.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: name})
	if err != nil {
		t.Fatalf("get data network %q after create: %v", name, err)
	}

	if got.Name != createOpts.Name ||
		got.IPv4Pool != createOpts.IPv4Pool ||
		got.IPv6Pool != createOpts.IPv6Pool ||
		got.DNS != createOpts.DNS ||
		got.Mtu != createOpts.Mtu {
		t.Fatalf("post-create round-trip mismatch: got %+v, want %+v", got, createOpts)
	}

	afterCreate := listAll()
	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if !contains(afterCreate.Items, name) {
		t.Fatalf("list after create missing %q", name)
	}

	// Updates round-trip every settable field independently. The base spec
	// is the post-create state; each case mutates exactly one field.
	base := *createOpts

	updateCases := []struct {
		field  string
		mutate func(o *client.UpdateDataNetworkOptions)
		assert func(t *testing.T, dn *client.DataNetwork)
	}{
		{
			field: "IPv4Pool",
			mutate: func(o *client.UpdateDataNetworkOptions) {
				o.IPv4Pool = "10.251.0.0/16"
			},
			assert: func(t *testing.T, dn *client.DataNetwork) {
				if dn.IPv4Pool != "10.251.0.0/16" {
					t.Fatalf("IPv4Pool: got %q, want %q", dn.IPv4Pool, "10.251.0.0/16")
				}
			},
		},
		{
			field: "IPv6Pool",
			mutate: func(o *client.UpdateDataNetworkOptions) {
				o.IPv6Pool = "fd9a::/48"
			},
			assert: func(t *testing.T, dn *client.DataNetwork) {
				if dn.IPv6Pool != "fd9a::/48" {
					t.Fatalf("IPv6Pool: got %q, want %q", dn.IPv6Pool, "fd9a::/48")
				}
			},
		},
		{
			field: "DNS",
			mutate: func(o *client.UpdateDataNetworkOptions) {
				o.DNS = "1.1.1.1"
			},
			assert: func(t *testing.T, dn *client.DataNetwork) {
				if dn.DNS != "1.1.1.1" {
					t.Fatalf("DNS: got %q, want %q", dn.DNS, "1.1.1.1")
				}
			},
		},
		{
			field: "Mtu",
			mutate: func(o *client.UpdateDataNetworkOptions) {
				o.Mtu = 1400
			},
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

			if err := c.UpdateDataNetwork(ctx, &opts); err != nil {
				t.Fatalf("update data network %q (%s): %v", name, tc.field, err)
			}

			updated, err := c.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: name})
			if err != nil {
				t.Fatalf("get data network %q after update: %v", name, err)
			}

			tc.assert(t, updated)
		})
	}

	if err := c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: name}); err != nil {
		t.Fatalf("delete data network %q: %v", name, err)
	}

	deleted = true

	_, err = c.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: name})
	assertNotFound(t, err, "data network after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if contains(afterDelete.Items, name) {
		t.Fatalf("list after delete still contains %q", name)
	}
}
