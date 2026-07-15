// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

// An edit reaches the same encoders as a create, so it answers to the same
// bounds: the EPS ceiling of TS 24.008 §10.5.6.5B and the 2-octet Session-AMBR
// value of TS 24.501 §9.11.4.14. The profile named in the request decides which
// apply, so moving a policy to a profile that allows 4G is a tightening move.
func TestUpdatePolicyBitrateRATLimits(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	url := env.Server.URL
	yes, no := true, false

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{
		Name: "prof-both", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps",
	}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{
		Name: "prof-5gonly", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps",
		Allow4G: &no, Allow5G: &yes,
	}); err != nil {
		t.Fatalf("create 5g-only profile: %s", err)
	}

	if _, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
		Name: "dn-upd", MTU: 1500, IPv4Pool: "10.60.0.0/24", DNS: "8.8.8.8",
	}); err != nil {
		t.Fatalf("create dn: %s", err)
	}

	if status, _, err := createPolicy(url, client, token, &CreatePolicyParams{
		Name: "pol-upd", ProfileName: "prof-5gonly", SliceName: DefaultSliceName,
		DataNetworkName: "dn-upd", SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps",
		Var5qi: 9, Arp: 1,
	}); err != nil || status != http.StatusCreated {
		t.Fatalf("seed policy: status %d, err %v", status, err)
	}

	cases := []struct {
		name    string
		profile string
		bitrate string
		status  int
	}{
		{"1500 Mbps on a 4G+5G profile", "prof-both", "1500 Mbps", http.StatusOK},
		{"10 Gbps on a 4G+5G profile (at the EPS ceiling)", "prof-both", "10 Gbps", http.StatusOK},
		{"20 Gbps on a 4G+5G profile is rejected (EPS clamps)", "prof-both", "20 Gbps", http.StatusBadRequest},
		// No regression on the edit path either: 5G-only keeps its headroom.
		{"20 Gbps on a 5G-only profile is allowed", "prof-5gonly", "20 Gbps", http.StatusOK},
		{"100000 Mbps is rejected (5G value > 65535)", "prof-5gonly", "100000 Mbps", http.StatusBadRequest},
		// The target profile decides, not the one the policy had.
		{"moving a 20 Gbps policy onto a 4G-capable profile is rejected", "prof-both", "20 Gbps", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _, err := editPolicy(url, client, "pol-upd", token, &UpdatePolicyParams{
				ProfileName: tc.profile, SliceName: DefaultSliceName, DataNetworkName: "dn-upd",
				SessionAmbrUplink: tc.bitrate, SessionAmbrDownlink: tc.bitrate,
				Var5qi: 9, Arp: 1,
			})
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			if status != tc.status {
				t.Errorf("%s @ %q: expected %d, got %d", tc.profile, tc.bitrate, tc.status, status)
			}
		})
	}
}

// The binding rules of TS 29.503 and TS 29.272 §7.3.35 bind an edit too: a
// policy may be pointed at a different data network, but not onto one already
// bound in the same slice.
func TestUpdatePolicyDataNetworkBinding(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	url := env.Server.URL
	no := false

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{
		Name: "prof-5g", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps", Allow4G: &no,
	}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	for _, dn := range []struct{ name, pool string }{
		{"dn-x", "10.70.0.0/24"}, {"dn-y", "10.71.0.0/24"}, {"dn-z", "10.72.0.0/24"},
	} {
		if _, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
			Name: dn.name, MTU: 1500, IPv4Pool: dn.pool, DNS: "8.8.8.8",
		}); err != nil {
			t.Fatalf("create dn %s: %s", dn.name, err)
		}
	}

	for _, p := range []struct{ name, dn string }{{"px", "dn-x"}, {"py", "dn-y"}} {
		if status, _, err := createPolicy(url, client, token, &CreatePolicyParams{
			Name: p.name, ProfileName: "prof-5g", SliceName: DefaultSliceName, DataNetworkName: p.dn,
			SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1,
		}); err != nil || status != http.StatusCreated {
			t.Fatalf("seed policy %s: status %d, err %v", p.name, status, err)
		}
	}

	edit := func(t *testing.T, policy, dn string, want int) {
		t.Helper()

		status, _, err := editPolicy(url, client, policy, token, &UpdatePolicyParams{
			ProfileName: "prof-5g", SliceName: DefaultSliceName, DataNetworkName: dn,
			SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1,
		})
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if status != want {
			t.Errorf("editing %s onto %s: expected %d, got %d", policy, dn, want, status)
		}
	}

	t.Run("moving a policy onto a free data network is accepted", func(t *testing.T) {
		edit(t, "px", "dn-z", http.StatusOK)
	})

	t.Run("moving a policy onto a data network bound in the same slice is rejected", func(t *testing.T) {
		edit(t, "px", "dn-y", http.StatusConflict)
	})

	// A policy must not collide with itself: its own binding is the one it holds.
	t.Run("editing a policy while keeping its own data network is accepted", func(t *testing.T) {
		edit(t, "py", "dn-y", http.StatusOK)
	})
}
