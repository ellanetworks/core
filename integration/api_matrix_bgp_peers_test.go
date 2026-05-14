package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBGPPeersMatrix runs without enabling the BGP speaker; peers are
// stored regardless of the speaker state. The Create response does not
// echo the assigned ID, so we recover it by listing and matching Address.
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

	// TEST-NET-1 (RFC 5737); not routable, so no BGP session ever forms.
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

	if created.HasPassword {
		t.Fatalf("HasPassword: got true, want false (peer was created without a password)")
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
		{
			// Password is *string with three semantics: nil leaves it
			// unchanged, &"value" sets it, &"" clears it. The server never
			// echoes the password back, so HasPassword is the witness.
			field: "Password_set",
			mutate: func(o *client.UpdateBGPPeerOptions) {
				secret := "topsecret"
				o.Password = &secret
			},
			assert: func(t *testing.T, p *client.BGPPeer) {
				if !p.HasPassword {
					t.Fatalf("HasPassword after set: got false, want true")
				}
			},
		},
		{
			field: "Password_clear",
			mutate: func(o *client.UpdateBGPPeerOptions) {
				empty := ""
				o.Password = &empty
			},
			assert: func(t *testing.T, p *client.BGPPeer) {
				if p.HasPassword {
					t.Fatalf("HasPassword after clear: got true, want false")
				}
			},
		},
	}

	for _, tc := range updateCases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			current, err := c.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID})
			if err != nil {
				t.Fatalf("get bgp peer %d before update: %v", created.ID, err)
			}

			opts := updateOptsFromBGPPeer(current)
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

// updateOptsFromBGPPeer mirrors current Get state into Update options.
// Password is left nil ("leave unchanged") since the server doesn't
// expose the current password.
func updateOptsFromBGPPeer(p *client.BGPPeer) client.UpdateBGPPeerOptions {
	return client.UpdateBGPPeerOptions{
		ID:             p.ID,
		Address:        p.Address,
		RemoteAS:       p.RemoteAS,
		HoldTime:       p.HoldTime,
		Description:    p.Description,
		ImportPrefixes: p.ImportPrefixes,
	}
}
