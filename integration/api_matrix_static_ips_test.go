// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runStaticIpsMatrix provisions the dependency chain a static IP requires
// (slice → data network → profile → policy → subscriber) then exercises the
// static-IP CRUD against a live core: create IPv4 + IPv6 pins, list, repin,
// delete, and the reject paths (duplicate family, out-of-pool address).
func runStaticIpsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	sliceName := apiMatrixName("staticip-slice")
	dnName := apiMatrixName("staticip-dn")
	profile := apiMatrixName("staticip-profile")
	policy := apiMatrixName("staticip-policy")
	imsi := "001019999999992"

	if err := c.CreateSlice(ctx, &client.CreateSliceOptions{Name: sliceName, Sst: 1, Sd: "abcdef"}); err != nil {
		t.Fatalf("create dep slice: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: sliceName}); err != nil {
			t.Logf("cleanup: delete dep slice: %v", err)
		}
	})

	if err := c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name: dnName, IPv4Pool: "10.252.0.0/16", IPv6Pool: "fd98::/48", DNS: "8.8.8.8", Mtu: 1500,
	}); err != nil {
		t.Fatalf("create dep data network: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: dnName}); err != nil {
			t.Logf("cleanup: delete dep data network: %v", err)
		}
	})

	if err := c.CreateProfile(ctx, &client.CreateProfileOptions{
		Name: profile, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps",
	}); err != nil {
		t.Fatalf("create dep profile: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: profile}); err != nil {
			t.Logf("cleanup: delete dep profile: %v", err)
		}
	})

	if err := c.CreatePolicy(ctx, &client.CreatePolicyOptions{
		Name: policy, ProfileName: profile, SliceName: sliceName, DataNetworkName: dnName,
		SessionAmbrUplink: "50 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 8,
	}); err != nil {
		t.Fatalf("create dep policy: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: policy}); err != nil {
			t.Logf("cleanup: delete dep policy: %v", err)
		}
	})

	if err := c.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi: imsi, Key: "640f441067cd56f1474cbcacd7a0588f", SequenceNumber: "000000000022",
		ProfileName: profile, OPc: "cb698a2341629c3241ae01de9d89de4f",
	}); err != nil {
		t.Fatalf("create dep subscriber: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
			t.Logf("cleanup: delete dep subscriber: %v", err)
		}
	})

	list := func() []client.StaticIP {
		resp, err := c.ListDataNetworkStaticIps(ctx, dnName)
		if err != nil {
			t.Fatalf("list static IPs: %v", err)
		}

		return resp.Items
	}

	find := func(items []client.StaticIP, ipVersion string) *client.StaticIP {
		for i := range items {
			if items[i].IPVersion == ipVersion {
				return &items[i]
			}
		}

		return nil
	}

	if n := len(list()); n != 0 {
		t.Fatalf("expected no static IPs before create, got %d", n)
	}

	if err := c.CreateDataNetworkStaticIp(ctx, dnName, &client.CreateStaticIPOptions{IMSI: imsi, Address: "10.252.0.10"}); err != nil {
		t.Fatalf("create IPv4 static IP: %v", err)
	}

	v4 := find(list(), "ipv4")
	if v4 == nil {
		t.Fatal("IPv4 static IP not listed after create")
	}

	if v4.IMSI != imsi || v4.Address != "10.252.0.10" || v4.Status != "reserved" || v4.SessionID != nil {
		t.Fatalf("unexpected IPv4 reservation: %+v", *v4)
	}

	// A second family for the same subscriber coexists.
	if err := c.CreateDataNetworkStaticIp(ctx, dnName, &client.CreateStaticIPOptions{IMSI: imsi, Address: "fd98:0:0:1::"}); err != nil {
		t.Fatalf("create IPv6 static IP: %v", err)
	}

	if n := len(list()); n != 2 {
		t.Fatalf("expected 2 static IPs (v4+v6), got %d", n)
	}

	// A duplicate for the same (imsi, family) is rejected.
	if err := c.CreateDataNetworkStaticIp(ctx, dnName, &client.CreateStaticIPOptions{IMSI: imsi, Address: "10.252.0.11"}); err == nil {
		t.Fatal("expected duplicate IPv4 static IP to be rejected")
	}

	// An address outside the pool is rejected.
	if err := c.CreateDataNetworkStaticIp(ctx, dnName, &client.CreateStaticIPOptions{IMSI: imsi, Address: "10.99.0.1"}); err == nil {
		t.Fatal("expected out-of-pool static IP to be rejected")
	}

	// Repin the IPv4 reservation.
	if err := c.UpdateDataNetworkStaticIp(ctx, dnName, imsi, "ipv4", "10.252.0.20"); err != nil {
		t.Fatalf("update IPv4 static IP: %v", err)
	}

	if got := find(list(), "ipv4"); got == nil || got.Address != "10.252.0.20" {
		t.Fatalf("repin not reflected: %+v", got)
	}

	// Delete the IPv4 reservation; the IPv6 one remains.
	if err := c.DeleteDataNetworkStaticIp(ctx, dnName, imsi, "ipv4"); err != nil {
		t.Fatalf("delete IPv4 static IP: %v", err)
	}

	items := list()
	if len(items) != 1 || find(items, "ipv6") == nil {
		t.Fatalf("expected only the IPv6 reservation after delete, got %+v", items)
	}

	if err := c.DeleteDataNetworkStaticIp(ctx, dnName, imsi, "ipv6"); err != nil {
		t.Fatalf("delete IPv6 static IP: %v", err)
	}

	if n := len(list()); n != 0 {
		t.Fatalf("expected no static IPs after delete, got %d", n)
	}
}
