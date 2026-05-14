package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runSubscribersMatrix exercises CRUD for subscribers. Subscribers
// reference a Profile, and the server enforces that any Profile carrying
// subscribers must be referenced by at least one Policy (see the 409
// "Profile has no policy" path in CreateSubscriber). The runner sets up
// the full dependency chain — one Slice, one Data Network, two Profiles,
// and one Policy per Profile — so it can also round-trip the
// profile_name field on Update.
//
// See api_matrix_profiles_test.go for the matrix shape. The only
// updatable field on a subscriber is profile_name (see
// internal/api/server/api_subscribers.go:28-30), so the update step has
// a single case.
func runSubscribersMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	sliceName := apiMatrixName("sub-slice")
	dnName := apiMatrixName("sub-dn")
	profileA := apiMatrixName("sub-profile-a")
	profileB := apiMatrixName("sub-profile-b")
	policyA := apiMatrixName("sub-policy-a")
	policyB := apiMatrixName("sub-policy-b")
	imsi := "001019999999991"

	if err := c.CreateSlice(ctx, &client.CreateSliceOptions{
		Name: sliceName,
		Sst:  1,
		Sd:   "abcdef",
	}); err != nil {
		t.Fatalf("create dep slice: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: sliceName}); err != nil {
			t.Logf("cleanup: delete dep slice: %v", err)
		}
	})

	if err := c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name:     dnName,
		IPv4Pool: "10.253.0.0/16",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}); err != nil {
		t.Fatalf("create dep data network: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: dnName}); err != nil {
			t.Logf("cleanup: delete dep data network: %v", err)
		}
	})

	for _, p := range []string{profileA, profileB} {
		p := p

		if err := c.CreateProfile(ctx, &client.CreateProfileOptions{
			Name:           p,
			UeAmbrUplink:   "100 Mbps",
			UeAmbrDownlink: "100 Mbps",
		}); err != nil {
			t.Fatalf("create dep profile %q: %v", p, err)
		}

		t.Cleanup(func() {
			if err := c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: p}); err != nil {
				t.Logf("cleanup: delete dep profile %q: %v", p, err)
			}
		})
	}

	for _, link := range []struct{ policy, profile string }{
		{policyA, profileA},
		{policyB, profileB},
	} {
		link := link

		if err := c.CreatePolicy(ctx, &client.CreatePolicyOptions{
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
			if err := c.DeletePolicy(ctx, &client.DeletePolicyOptions{Name: link.policy}); err != nil {
				t.Logf("cleanup: delete dep policy %q: %v", link.policy, err)
			}
		})
	}

	listAll := func() *client.ListSubscribersResponse {
		resp, err := c.ListSubscribers(ctx, &client.ListSubscribersParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list subscribers: %v", err)
		}

		return resp
	}

	contains := func(items []client.Subscriber, imsi string) bool {
		for _, s := range items {
			if s.Imsi == imsi {
				return true
			}
		}

		return false
	}

	baseline := listAll()

	createOpts := &client.CreateSubscriberOptions{
		Imsi:           imsi,
		Key:            "640f441067cd56f1474cbcacd7a0588f",
		SequenceNumber: "000000000022",
		ProfileName:    profileA,
		OPc:            "cb698a2341629c3241ae01de9d89de4f",
	}

	if err := c.CreateSubscriber(ctx, createOpts); err != nil {
		t.Fatalf("create subscriber %q: %v", imsi, err)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
			t.Logf("cleanup: delete subscriber %q: %v", imsi, err)
		}
	})

	got, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
	if err != nil {
		t.Fatalf("get subscriber %q after create: %v", imsi, err)
	}

	if got.Imsi != imsi || got.ProfileName != profileA {
		t.Fatalf("post-create round-trip mismatch: got %+v, want imsi=%s profile=%s", got, imsi, profileA)
	}

	// Credentials are stored at create and exposed via a separate endpoint;
	// round-trip key and opc as part of the Read step.
	creds, err := c.GetSubscriberCredentials(ctx, &client.GetSubscriberCredentialsOptions{ID: imsi})
	if err != nil {
		t.Fatalf("get subscriber credentials: %v", err)
	}

	if creds.Key != createOpts.Key {
		t.Fatalf("credentials Key: got %q, want %q", creds.Key, createOpts.Key)
	}

	if creds.Opc != createOpts.OPc {
		t.Fatalf("credentials Opc: got %q, want %q", creds.Opc, createOpts.OPc)
	}

	afterCreate := listAll()
	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if !contains(afterCreate.Items, imsi) {
		t.Fatalf("list after create missing %q", imsi)
	}

	t.Run("update_ProfileName", func(t *testing.T) {
		if err := c.UpdateSubscriber(ctx, imsi, &client.UpdateSubscriberOptions{ProfileName: profileB}); err != nil {
			t.Fatalf("update subscriber: %v", err)
		}

		updated, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		if err != nil {
			t.Fatalf("get subscriber after update: %v", err)
		}

		if updated.ProfileName != profileB {
			t.Fatalf("ProfileName: got %q, want %q", updated.ProfileName, profileB)
		}
	})

	if err := c.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
		t.Fatalf("delete subscriber %q: %v", imsi, err)
	}

	deleted = true

	_, err = c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
	assertNotFound(t, err, "subscriber after delete")

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if contains(afterDelete.Items, imsi) {
		t.Fatalf("list after delete still contains %q", imsi)
	}
}
