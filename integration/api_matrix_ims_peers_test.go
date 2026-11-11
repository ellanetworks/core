// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

type imsPeersStep struct {
	name   string
	writer int
	apply  func(c *client.Client) error
	want   []client.AllowedIMSPeer
}

var (
	imsPeerByHost  = apiMatrixName("ims-peer-host")
	imsPeerByRealm = apiMatrixName("ims-peer-realm")
)

func imsPeersMatrixSteps(ctx context.Context) []imsPeersStep {
	byHost := client.AllowedIMSPeer{Name: imsPeerByHost, OriginHost: "scscf.apimat.example.org", Addresses: []string{"198.51.100.0/24"}, Connections: []client.DiameterPeerStatus{}}
	byRealm := client.AllowedIMSPeer{Name: imsPeerByRealm, Realm: "apimat.example.org", Addresses: []string{"192.0.2.0/24", "2001:db8::1/128"}, Connections: []client.DiameterPeerStatus{}}
	updated := client.AllowedIMSPeer{Name: imsPeerByHost, OriginHost: "icscf.apimat.example.org", Realm: "apimat.example.net", Addresses: []string{"198.51.100.7/32"}, Connections: []client.DiameterPeerStatus{}}

	return []imsPeersStep{
		{
			name:   "create_by_origin_host",
			writer: 0,
			apply: func(c *client.Client) error {
				_, err := c.CreateAllowedIMSPeer(ctx, &client.CreateAllowedIMSPeerOptions{Name: byHost.Name, OriginHost: byHost.OriginHost, Addresses: byHost.Addresses})
				return err
			},
			want: []client.AllowedIMSPeer{byHost},
		},
		{
			name:   "create_by_realm",
			writer: 1,
			apply: func(c *client.Client) error {
				_, err := c.CreateAllowedIMSPeer(ctx, &client.CreateAllowedIMSPeerOptions{Name: byRealm.Name, Realm: byRealm.Realm, Addresses: []string{"2001:db8::1", "192.0.2.9/24"}})
				return err
			},
			want: []client.AllowedIMSPeer{byHost, byRealm},
		},
		{
			name:   "update",
			writer: 2,
			apply: func(c *client.Client) error {
				return c.UpdateAllowedIMSPeer(ctx, updated.Name, &client.UpdateAllowedIMSPeerOptions{OriginHost: updated.OriginHost, Realm: updated.Realm, Addresses: []string{"198.51.100.7"}})
			},
			want: []client.AllowedIMSPeer{updated, byRealm},
		},
		{
			name:   "delete_by_realm",
			writer: 0,
			apply: func(c *client.Client) error {
				return c.DeleteAllowedIMSPeer(ctx, byRealm.Name)
			},
			want: []client.AllowedIMSPeer{updated},
		},
		{
			name:   "delete_by_origin_host",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.DeleteAllowedIMSPeer(ctx, updated.Name)
			},
			want: []client.AllowedIMSPeer{},
		},
	}
}

func readIMSPeers(ctx context.Context, t *testing.T, c *client.Client) []client.AllowedIMSPeer {
	t.Helper()

	peers, err := c.ListAllowedIMSPeers(ctx)
	if err != nil {
		t.Fatalf("list IMS peers: %v", err)
	}

	out := []client.AllowedIMSPeer{}

	for _, p := range peers.Items {
		if strings.HasPrefix(p.Name, apiMatrixName("")) {
			out = append(out, p)
		}
	}

	return out
}

func assertIMSPeers(ctx context.Context, t *testing.T, c *client.Client, node int, want []client.AllowedIMSPeer) {
	t.Helper()

	got := readIMSPeers(ctx, t, c)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("node %d IMS peers: got %+v, want %+v", node, got, want)
	}

	for _, w := range want {
		p, err := c.GetAllowedIMSPeer(ctx, w.Name)
		if err != nil {
			t.Fatalf("node %d get IMS peer %s: %v", node, w.Name, err)
		}

		if !reflect.DeepEqual(*p, w) {
			t.Fatalf("node %d IMS peer %s: got %+v, want %+v", node, w.Name, *p, w)
		}
	}

	for _, name := range []string{imsPeerByHost, imsPeerByRealm} {
		if !containsIMSPeer(want, name) {
			_, err := c.GetAllowedIMSPeer(ctx, name)
			assertNotFound(t, err, "IMS peer "+name)
		}
	}

	settings, err := c.GetIMSSettings(ctx)
	if err != nil {
		t.Fatalf("node %d get IMS settings: %v", node, err)
	}

	if settings.RestrictPeers {
		t.Fatalf("node %d IMS peers restricted, want unrestricted: allowed IMS peers alone do not restrict", node)
	}
}

func containsIMSPeer(peers []client.AllowedIMSPeer, name string) bool {
	for _, p := range peers {
		if p.Name == name {
			return true
		}
	}

	return false
}

func cleanupIMSPeers(ctx context.Context, t *testing.T, c *client.Client) {
	peers, err := c.ListAllowedIMSPeers(ctx)
	if err != nil {
		t.Logf("cleanup: list IMS peers: %v", err)
		return
	}

	for _, p := range peers.Items {
		if strings.HasPrefix(p.Name, apiMatrixName("")) {
			if err := c.DeleteAllowedIMSPeer(ctx, p.Name); err != nil {
				t.Logf("cleanup: delete IMS peer %s: %v", p.Name, err)
			}
		}
	}
}

func runIMSPeersMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Cleanup(func() { cleanupIMSPeers(ctx, t, c) })

	for _, step := range imsPeersMatrixSteps(ctx) {
		t.Run(step.name, func(t *testing.T) {
			if err := step.apply(c); err != nil {
				t.Fatalf("%s: %v", step.name, err)
			}

			assertIMSPeers(ctx, t, c, 1, step.want)
		})
	}

	t.Run("rejects_invalid", func(t *testing.T) {
		name := apiMatrixName("ims-peer-invalid")

		invalid := []client.CreateAllowedIMSPeerOptions{
			{Name: name},
			{Name: name, OriginHost: "not a host"},
			{Name: name, Realm: "localhost"},
			{Name: name, Realm: "apimat.example.org", Addresses: []string{"not-an-address"}},
			{Name: name, Addresses: []string{"192.0.2.1"}},
			{Name: name, OriginHost: "scscf.apimat.example.org"},
		}

		for _, opts := range invalid {
			if _, err := c.CreateAllowedIMSPeer(ctx, &opts); err == nil {
				t.Fatalf("create IMS peer %+v: want an error", opts)
			}
		}

		_, err := c.GetAllowedIMSPeer(ctx, name)
		assertNotFound(t, err, "IMS peer "+name)
	})

	t.Run("rejects_duplicates", func(t *testing.T) {
		host := "dup.apimat.example.org"

		if _, err := c.CreateAllowedIMSPeer(ctx, &client.CreateAllowedIMSPeerOptions{Name: imsPeerByHost, OriginHost: host, Addresses: []string{"::/0"}}); err != nil {
			t.Fatalf("create IMS peer: %v", err)
		}

		if _, err := c.CreateAllowedIMSPeer(ctx, &client.CreateAllowedIMSPeerOptions{Name: imsPeerByHost, Realm: "apimat.example.org", Addresses: []string{"::/0"}}); err == nil {
			t.Fatalf("create IMS peer with a duplicate name: want an error")
		}

		if _, err := c.CreateAllowedIMSPeer(ctx, &client.CreateAllowedIMSPeerOptions{Name: imsPeerByRealm, OriginHost: strings.ToUpper(host), Addresses: []string{"::/0"}}); err == nil {
			t.Fatalf("create IMS peer with a duplicate Origin-Host: want an error")
		}

		if err := c.DeleteAllowedIMSPeer(ctx, imsPeerByHost); err != nil {
			t.Fatalf("delete IMS peer: %v", err)
		}

		assertIMSPeers(ctx, t, c, 1, []client.AllowedIMSPeer{})
	})
}

func runIMSPeersHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	t.Cleanup(func() { cleanupIMSPeers(ctx, t, h.Leader) })

	for _, step := range imsPeersMatrixSteps(ctx) {
		t.Run(step.name, func(t *testing.T) {
			if err := step.apply(nodes[step.writer]); err != nil {
				t.Fatalf("%s on node %d: %v", step.name, step.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				assertIMSPeers(ctx, t, c, i+1, step.want)
			}
		})
	}
}
