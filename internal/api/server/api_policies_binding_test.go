// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

// A data network may back one policy per slice (TS 29.503 keys a slice's DNN
// configurations by DNN), and only one per profile once 4G is allowed
// (TS 29.272 §7.3.35 requires a unique APN per subscriber).
func TestPolicyDataNetworkBinding(t *testing.T) {
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

	if _, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
		Name: "dn-a", MTU: 1500, IPv4Pool: "10.10.0.0/24", DNS: "8.8.8.8",
	}); err != nil {
		t.Fatalf("create data network: %s", err)
	}

	if _, _, err := createSlice(url, client, token, &CreateSliceParams{Name: "slice-b", Sst: 1, Sd: "010203"}); err != nil {
		t.Fatalf("create slice: %s", err)
	}

	// 5G-only profile.
	no := false
	if _, _, err := createProfile(url, client, token, &CreateProfileParams{
		Name: "prof-5g", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps", Allow4G: &no,
	}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	mk := func(name, slice, dn string) *CreatePolicyParams {
		return &CreatePolicyParams{
			Name: name, ProfileName: "prof-5g", SliceName: slice, DataNetworkName: dn,
			SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1,
		}
	}

	t.Run("first binding is accepted", func(t *testing.T) {
		status, _, err := createPolicy(url, client, token, mk("p1", DefaultSliceName, "dn-a"))
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if status != http.StatusCreated {
			t.Errorf("expected 201, got %d", status)
		}
	})

	t.Run("same data network in the SAME slice is rejected", func(t *testing.T) {
		status, _, err := createPolicy(url, client, token, mk("p2", DefaultSliceName, "dn-a"))
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if status != http.StatusConflict {
			t.Errorf("expected 409, got %d", status)
		}
	})

	// The load-bearing case: legal 5G, must NOT be rejected.
	t.Run("same data network in a DIFFERENT slice is allowed on a 5G-only profile", func(t *testing.T) {
		status, _, err := createPolicy(url, client, token, mk("p3", "slice-b", "dn-a"))
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if status != http.StatusCreated {
			t.Errorf("expected 201 (legal per TS 29.503: DNN keys configs per slice), got %d", status)
		}
	})

	t.Run("enabling 4G on that profile is rejected while the duplicate exists", func(t *testing.T) {
		yes := true

		status, _, err := editProfile(url, client, "prof-5g", token, &UpdateProfileParams{
			UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps", Allow4G: &yes,
		})
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if status != http.StatusBadRequest {
			t.Errorf("expected 400 (TS 29.272 §7.3.35: unique APN per subscriber), got %d", status)
		}
	})
}
