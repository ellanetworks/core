package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runSubscribersHAMatrix exercises subscriber CRUD across the 3-node
// cluster, rotating the writer node so both leader-direct writes and
// follower-proxy writes are covered. After every mutation it asserts
// the resulting state on every node.
func runSubscribersHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	if len(h.Clients) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(h.Clients))
	}

	sliceName := apiMatrixName("ha-sub-slice")
	dnName := apiMatrixName("ha-sub-dn")
	profileA := apiMatrixName("ha-sub-profile-a")
	profileB := apiMatrixName("ha-sub-profile-b")
	policyA := apiMatrixName("ha-sub-policy-a")
	policyB := apiMatrixName("ha-sub-policy-b")
	imsi := "001019999999991"

	leader := h.Leader
	nodes := h.Clients

	if err := leader.CreateSlice(ctx, &client.CreateSliceOptions{
		Name: sliceName,
		Sst:  1,
		Sd:   "abcdef",
	}); err != nil {
		t.Fatalf("create dep slice: %v", err)
	}

	t.Cleanup(func() {
		if err := leader.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: sliceName}); err != nil {
			t.Logf("cleanup: delete dep slice: %v", err)
		}
	})

	if err := leader.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name:     dnName,
		IPv4Pool: "10.253.0.0/16",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}); err != nil {
		t.Fatalf("create dep data network: %v", err)
	}

	t.Cleanup(func() {
		if err := leader.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: dnName}); err != nil {
			t.Logf("cleanup: delete dep data network: %v", err)
		}
	})

	for _, p := range []string{profileA, profileB} {
		p := p

		if err := leader.CreateProfile(ctx, &client.CreateProfileOptions{
			Name:           p,
			UeAmbrUplink:   "100 Mbps",
			UeAmbrDownlink: "100 Mbps",
		}); err != nil {
			t.Fatalf("create dep profile %q: %v", p, err)
		}

		t.Cleanup(func() {
			if err := leader.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: p}); err != nil {
				t.Logf("cleanup: delete dep profile %q: %v", p, err)
			}
		})
	}

	for _, link := range []struct{ policy, profile string }{
		{policyA, profileA},
		{policyB, profileB},
	} {
		link := link

		if err := leader.CreatePolicy(ctx, &client.CreatePolicyOptions{
			Name:                link.policy,
			ProfileName:         link.profile,
			SliceName:           sliceName,
			DataNetworkName:     dnName,
			SessionAmbrUplink:   "50 Mbps",
			SessionAmbrDownlink: "100 Mbps",
			Var5qi:              9,
			Arp:                 8,
		}); err != nil {
			t.Fatalf("create dep policy %q: %v", link.policy, err)
		}

		t.Cleanup(func() {
			if err := leader.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: link.policy}); err != nil {
				t.Logf("cleanup: delete dep policy %q: %v", link.policy, err)
			}
		})
	}

	// Without this barrier the first follower-write could race against
	// dep replication and fail with a 404 on the policy lookup.
	awaitConvergence(ctx, t, h)

	createOpts := &client.CreateSubscriberOptions{
		Imsi:           imsi,
		Key:            "640f441067cd56f1474cbcacd7a0588f",
		SequenceNumber: "000000000022",
		ProfileName:    profileA,
		OPc:            "cb698a2341629c3241ae01de9d89de4f",
	}

	if err := nodes[0].CreateSubscriber(ctx, createOpts); err != nil {
		t.Fatalf("create subscriber on node 1: %v", err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := leader.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
			t.Logf("cleanup: delete subscriber: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		if err != nil {
			t.Fatalf("get subscriber on node %d after create: %v", i+1, err)
		}

		if got.Imsi != imsi || got.ProfileName != profileA {
			t.Fatalf("node %d: got imsi=%q profile=%q, want imsi=%q profile=%q",
				i+1, got.Imsi, got.ProfileName, imsi, profileA)
		}

		list, err := c.ListSubscribers(ctx, &client.ListSubscribersParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list subscribers on node %d after create: %v", i+1, err)
		}

		if !subscribersContains(list.Items, imsi) {
			t.Fatalf("node %d list missing %q after create", i+1, imsi)
		}
	}

	if err := nodes[1].UpdateSubscriber(ctx, imsi, &client.UpdateSubscriberOptions{ProfileName: profileB}); err != nil {
		t.Fatalf("update subscriber on node 2: %v", err)
	}

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		if err != nil {
			t.Fatalf("get subscriber on node %d after update: %v", i+1, err)
		}

		if got.ProfileName != profileB {
			t.Fatalf("node %d ProfileName after update: got %q, want %q", i+1, got.ProfileName, profileB)
		}
	}

	if err := nodes[2].DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
		t.Fatalf("delete subscriber on node 3: %v", err)
	}

	deleted = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		_, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		assertNotFound(t, err, fmt.Sprintf("subscriber on node %d after delete", i+1))

		list, err := c.ListSubscribers(ctx, &client.ListSubscribersParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list subscribers on node %d after delete: %v", i+1, err)
		}

		if subscribersContains(list.Items, imsi) {
			t.Fatalf("node %d list still contains %q after delete", i+1, imsi)
		}
	}
}

func subscribersContains(items []client.Subscriber, imsi string) bool {
	for _, s := range items {
		if s.Imsi == imsi {
			return true
		}
	}

	return false
}
