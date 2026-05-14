package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBGPPeersMatrix exercises CRUD for BGP peers. The handler at
// internal/api/server/api_bgp.go:435 does not require BGP to be enabled
// for create — peers are stored in the DB regardless — so we can run
// this matrix without first toggling BGP settings.
//
// Peers are identified by integer ID assigned at create time; since the
// Create response does not echo the ID back, we List immediately after
// Create and look up our peer by Address.
func runBGPPeersMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	listAll := func() *client.ListBGPPeersResponse {
		resp, err := c.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list bgp peers: %v", err)
		}

		return resp
	}

	findByAddress := func(items []client.BGPPeer, addr string) *client.BGPPeer {
		for i := range items {
			if items[i].Address == addr {
				return &items[i]
			}
		}

		return nil
	}

	baseline := listAll()

	// 192.0.2.0/24 is TEST-NET-1 (RFC 5737); not routable, so the BGP
	// session never establishes — but the DB record exists.
	createOpts := &client.CreateBGPPeerOptions{
		Address:     "192.0.2.10",
		RemoteAS:    65000,
		HoldTime:    90,
		Description: "apimatrix peer",
		ImportPrefixes: []client.BGPImportPrefix{
			{Prefix: "10.0.0.0/8", MaxLength: 24},
		},
	}

	if err := c.CreateBGPPeer(ctx, createOpts); err != nil {
		t.Fatalf("create bgp peer: %v", err)
	}

	afterCreate := listAll()

	created := findByAddress(afterCreate.Items, createOpts.Address)
	if created == nil {
		t.Fatalf("list after create missing address %q", createOpts.Address)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: created.ID}); err != nil {
			t.Logf("cleanup: delete bgp peer %d: %v", created.ID, err)
		}
	})

	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if created.RemoteAS != createOpts.RemoteAS ||
		created.HoldTime != createOpts.HoldTime ||
		created.Description != createOpts.Description ||
		len(created.ImportPrefixes) != 1 ||
		created.ImportPrefixes[0].Prefix != "10.0.0.0/8" ||
		created.ImportPrefixes[0].MaxLength != 24 {
		t.Fatalf("post-create round-trip mismatch: got %+v, want %+v", created, createOpts)
	}

	got, err := c.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID})
	if err != nil {
		t.Fatalf("get bgp peer %d: %v", created.ID, err)
	}

	if got.ID != created.ID || got.Address != createOpts.Address {
		t.Fatalf("get bgp peer mismatch: got %+v, want id=%d address=%q", got, created.ID, createOpts.Address)
	}

	updateCases := []struct {
		field  string
		mutate func(o *client.UpdateBGPPeerOptions)
		assert func(t *testing.T, p *client.BGPPeer)
	}{
		{
			field: "RemoteAS",
			mutate: func(o *client.UpdateBGPPeerOptions) {
				o.RemoteAS = 65001
			},
			assert: func(t *testing.T, p *client.BGPPeer) {
				if p.RemoteAS != 65001 {
					t.Fatalf("RemoteAS: got %d, want 65001", p.RemoteAS)
				}
			},
		},
		{
			field: "HoldTime",
			mutate: func(o *client.UpdateBGPPeerOptions) {
				o.HoldTime = 120
			},
			assert: func(t *testing.T, p *client.BGPPeer) {
				if p.HoldTime != 120 {
					t.Fatalf("HoldTime: got %d, want 120", p.HoldTime)
				}
			},
		},
		{
			field: "Description",
			mutate: func(o *client.UpdateBGPPeerOptions) {
				o.Description = "apimatrix peer updated"
			},
			assert: func(t *testing.T, p *client.BGPPeer) {
				if p.Description != "apimatrix peer updated" {
					t.Fatalf("Description: got %q, want %q", p.Description, "apimatrix peer updated")
				}
			},
		},
		{
			field: "ImportPrefixes",
			mutate: func(o *client.UpdateBGPPeerOptions) {
				o.ImportPrefixes = []client.BGPImportPrefix{
					{Prefix: "172.16.0.0/12", MaxLength: 20},
				}
			},
			assert: func(t *testing.T, p *client.BGPPeer) {
				if len(p.ImportPrefixes) != 1 ||
					p.ImportPrefixes[0].Prefix != "172.16.0.0/12" ||
					p.ImportPrefixes[0].MaxLength != 20 {
					t.Fatalf("ImportPrefixes: got %+v, want [172.16.0.0/12 maxLength=20]", p.ImportPrefixes)
				}
			},
		},
	}

	for _, tc := range updateCases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			opts := client.UpdateBGPPeerOptions{
				ID:             created.ID,
				Address:        createOpts.Address,
				RemoteAS:       createOpts.RemoteAS,
				HoldTime:       createOpts.HoldTime,
				Description:    createOpts.Description,
				ImportPrefixes: createOpts.ImportPrefixes,
			}

			tc.mutate(&opts)

			if err := c.UpdateBGPPeer(ctx, &opts); err != nil {
				t.Fatalf("update bgp peer %d (%s): %v", created.ID, tc.field, err)
			}

			updated, err := c.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID})
			if err != nil {
				t.Fatalf("get bgp peer after update: %v", err)
			}

			tc.assert(t, updated)
		})
	}

	if err := c.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: created.ID}); err != nil {
		t.Fatalf("delete bgp peer %d: %v", created.ID, err)
	}

	deleted = true

	_, err = c.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID})
	assertNotFound(t, err, "bgp peer after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if findByAddress(afterDelete.Items, createOpts.Address) != nil {
		t.Fatalf("list after delete still contains address %q", createOpts.Address)
	}
}
