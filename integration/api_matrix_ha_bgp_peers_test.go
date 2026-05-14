package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBGPPeersHAMatrix exercises BGP peer CRUD on one node and asserts
// the peer never appears on the other two. Updates and password-set/clear
// also run on the writer; locality is rechecked after each mutation.
func runBGPPeersHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const writerIdx = 1

	writer := h.Clients[writerIdx]

	listAllOn := func(c *client.Client) *client.ListBGPPeersResponse {
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

	baselines := make(map[int]*client.ListBGPPeersResponse, len(h.Clients))
	for i, c := range h.Clients {
		baselines[i] = listAllOn(c)
	}

	createOpts := &client.CreateBGPPeerOptions{
		Address:     "192.0.2.10",
		RemoteAS:    65000,
		HoldTime:    90,
		Description: "apimatrix-ha peer",
		ImportPrefixes: []client.BGPImportPrefix{
			{Prefix: "10.0.0.0/8", MaxLength: 24},
		},
	}

	if err := writer.CreateBGPPeer(ctx, createOpts); err != nil {
		t.Fatalf("create bgp peer on node %d: %v", writerIdx+1, err)
	}

	writerList := listAllOn(writer)

	created := findByAddress(writerList.Items, createOpts.Address)
	if created == nil {
		t.Fatalf("writer node %d missing peer %q after create", writerIdx+1, createOpts.Address)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := writer.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: created.ID}); err != nil {
			t.Logf("cleanup: delete bgp peer %d: %v", created.ID, err)
		}
	})

	if writerList.TotalCount != baselines[writerIdx].TotalCount+1 {
		t.Fatalf("writer node %d count after create: got %d, want %d",
			writerIdx+1, writerList.TotalCount, baselines[writerIdx].TotalCount+1)
	}

	if created.RemoteAS != createOpts.RemoteAS ||
		created.HoldTime != createOpts.HoldTime ||
		created.Description != createOpts.Description ||
		len(created.ImportPrefixes) != 1 ||
		created.ImportPrefixes[0].Prefix != "10.0.0.0/8" ||
		created.ImportPrefixes[0].MaxLength != 24 {
		t.Fatalf("writer node %d post-create mismatch: got %+v, want %+v", writerIdx+1, created, createOpts)
	}

	if created.HasPassword {
		t.Fatalf("HasPassword: got true, want false (peer was created without a password)")
	}

	assertLocalityBGPPeer := func(t *testing.T, phase string) {
		t.Helper()

		stabilizeLocal()

		for i, c := range h.Clients {
			if i == writerIdx {
				continue
			}

			other := listAllOn(c)
			if other.TotalCount != baselines[i].TotalCount {
				t.Fatalf("node %d count after %s: got %d, want %d (peer leaked from node %d)",
					i+1, phase, other.TotalCount, baselines[i].TotalCount, writerIdx+1)
			}

			if findByAddress(other.Items, createOpts.Address) != nil {
				t.Fatalf("node %d list contains peer %q after writer-only %s (locality breach)",
					i+1, createOpts.Address, phase)
			}

			if _, err := c.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID}); err == nil {
				t.Fatalf("node %d returned peer id=%d created only on node %d",
					i+1, created.ID, writerIdx+1)
			}
		}
	}

	assertLocalityBGPPeer(t, "create")

	updateCases := []struct {
		field  string
		mutate func(o *client.UpdateBGPPeerOptions)
		assert func(t *testing.T, p *client.BGPPeer)
	}{
		{
			field:  "RemoteAS",
			mutate: func(o *client.UpdateBGPPeerOptions) { o.RemoteAS = 65001 },
			assert: func(t *testing.T, p *client.BGPPeer) {
				if p.RemoteAS != 65001 {
					t.Fatalf("RemoteAS: got %d, want 65001", p.RemoteAS)
				}
			},
		},
		{
			field:  "HoldTime",
			mutate: func(o *client.UpdateBGPPeerOptions) { o.HoldTime = 120 },
			assert: func(t *testing.T, p *client.BGPPeer) {
				if p.HoldTime != 120 {
					t.Fatalf("HoldTime: got %d, want 120", p.HoldTime)
				}
			},
		},
		{
			field:  "Description",
			mutate: func(o *client.UpdateBGPPeerOptions) { o.Description = "apimatrix-ha peer updated" },
			assert: func(t *testing.T, p *client.BGPPeer) {
				if p.Description != "apimatrix-ha peer updated" {
					t.Fatalf("Description: got %q, want %q", p.Description, "apimatrix-ha peer updated")
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
			current, err := writer.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID})
			if err != nil {
				t.Fatalf("get bgp peer %d on writer before update: %v", created.ID, err)
			}

			opts := updateOptsFromBGPPeer(current)
			tc.mutate(&opts)

			if err := writer.UpdateBGPPeer(ctx, &opts); err != nil {
				t.Fatalf("update bgp peer %d on writer (%s): %v", created.ID, tc.field, err)
			}

			updated, err := writer.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID})
			if err != nil {
				t.Fatalf("get bgp peer after update on writer: %v", err)
			}

			tc.assert(t, updated)

			assertLocalityBGPPeer(t, "update_"+tc.field)
		})
	}

	if err := writer.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: created.ID}); err != nil {
		t.Fatalf("delete bgp peer %d on writer: %v", created.ID, err)
	}

	deleted = true

	_, err := writer.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: created.ID})
	assertNotFound(t, err, "bgp peer on writer after delete")

	writerFinal := listAllOn(writer)
	if writerFinal.TotalCount != baselines[writerIdx].TotalCount {
		t.Fatalf("writer node %d count after delete: got %d, want %d",
			writerIdx+1, writerFinal.TotalCount, baselines[writerIdx].TotalCount)
	}

	if findByAddress(writerFinal.Items, createOpts.Address) != nil {
		t.Fatalf("writer node %d list still contains peer %q after delete",
			writerIdx+1, createOpts.Address)
	}
}
