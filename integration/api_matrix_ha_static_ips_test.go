// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runStaticIpsHAMatrix exercises static-IP CRUD across the 3-node cluster,
// rotating the writer node (create on node 1, repin on node 2, delete on
// node 3) and asserting every node converges on the reservation state.
func runStaticIpsHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	if len(h.Clients) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(h.Clients))
	}

	sliceName := apiMatrixName("ha-staticip-slice")
	dnName := apiMatrixName("ha-staticip-dn")
	profile := apiMatrixName("ha-staticip-profile")
	policy := apiMatrixName("ha-staticip-policy")
	imsi := "001019999999993"

	leader := h.Leader
	nodes := h.Clients

	if err := leader.CreateSlice(ctx, &client.CreateSliceOptions{Name: sliceName, Sst: 1, Sd: "abcdef"}); err != nil {
		t.Fatalf("create dep slice: %v", err)
	}

	t.Cleanup(func() {
		if err := leader.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: sliceName}); err != nil {
			t.Logf("cleanup: delete dep slice: %v", err)
		}
	})

	if err := leader.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name: dnName, IPv4Pool: "10.252.0.0/16", DNS: "8.8.8.8", Mtu: 1500,
	}); err != nil {
		t.Fatalf("create dep data network: %v", err)
	}

	t.Cleanup(func() {
		if err := leader.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: dnName}); err != nil {
			t.Logf("cleanup: delete dep data network: %v", err)
		}
	})

	if err := leader.CreateProfile(ctx, &client.CreateProfileOptions{
		Name: profile, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps",
	}); err != nil {
		t.Fatalf("create dep profile: %v", err)
	}

	t.Cleanup(func() {
		if err := leader.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profile}); err != nil {
			t.Logf("cleanup: delete dep profile: %v", err)
		}
	})

	if err := leader.CreatePolicy(ctx, &client.CreatePolicyOptions{
		Name: policy, ProfileName: profile, SliceName: sliceName, DataNetworkName: dnName,
		SessionAmbrUplink: "50 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 8,
	}); err != nil {
		t.Fatalf("create dep policy: %v", err)
	}

	t.Cleanup(func() {
		if err := leader.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: policy}); err != nil {
			t.Logf("cleanup: delete dep policy: %v", err)
		}
	})

	if err := leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi: imsi, Key: "640f441067cd56f1474cbcacd7a0588f", SequenceNumber: "000000000022",
		ProfileName: profile, OPc: "cb698a2341629c3241ae01de9d89de4f",
	}); err != nil {
		t.Fatalf("create dep subscriber: %v", err)
	}

	t.Cleanup(func() {
		if err := leader.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
			t.Logf("cleanup: delete dep subscriber: %v", err)
		}
	})

	// Barrier so the dependency chain replicates before the first
	// follower-proxied static-IP write validates against its local DB.
	awaitConvergence(ctx, t, h)

	findV4 := func(c *client.Client) *client.StaticIP {
		resp, err := c.ListDataNetworkStaticIps(ctx, dnName)
		if err != nil {
			t.Fatalf("list static IPs: %v", err)
		}

		for i := range resp.Items {
			if resp.Items[i].IPVersion == "ipv4" {
				return &resp.Items[i]
			}
		}

		return nil
	}

	if err := nodes[0].CreateDataNetworkStaticIp(ctx, dnName, &client.CreateStaticIPOptions{IMSI: imsi, Address: "10.252.0.10"}); err != nil {
		t.Fatalf("create static IP on node 1: %v", err)
	}

	removed := false

	t.Cleanup(func() {
		if removed {
			return
		}

		if err := leader.DeleteDataNetworkStaticIp(ctx, dnName, imsi, "ipv4"); err != nil {
			t.Logf("cleanup: delete static IP: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got := findV4(c)
		if got == nil {
			t.Fatalf("node %d: static IP missing after create", i+1)
		}

		if got.IMSI != imsi || got.Address != "10.252.0.10" || got.Status != "reserved" {
			t.Fatalf("node %d: unexpected reservation after create: %+v", i+1, *got)
		}
	}

	if err := nodes[1].UpdateDataNetworkStaticIp(ctx, dnName, imsi, "ipv4", "10.252.0.20"); err != nil {
		t.Fatalf("repin static IP on node 2: %v", err)
	}

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got := findV4(c)
		if got == nil || got.Address != "10.252.0.20" {
			t.Fatalf("node %d: repin not converged, got %+v", i+1, got)
		}
	}

	if err := nodes[2].DeleteDataNetworkStaticIp(ctx, dnName, imsi, "ipv4"); err != nil {
		t.Fatalf("delete static IP on node 3: %v", err)
	}

	removed = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		if findV4(c) != nil {
			t.Fatalf("node %d: static IP still present after delete", i+1)
		}
	}
}
