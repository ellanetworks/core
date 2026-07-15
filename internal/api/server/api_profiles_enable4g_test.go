// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

// Enabling 4G on a profile subjects its existing policies to the EPS bounds,
// which a 5G-only policy was never held to: EPS scales Session-AMBR in kbps and
// runs out of units above 10 Gbps (TS 24.008 §10.5.6.5B), where the encoder
// clamps. The transition is refused so the rate stays what the operator set.
func TestEnable4GRejectsUnencodableSessionAmbr(t *testing.T) {
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

	seed := func(t *testing.T, profile, dn, pool, ambr string) {
		t.Helper()

		if _, _, err := createProfile(url, client, token, &CreateProfileParams{
			Name: profile, UeAmbrUplink: "1 Gbps", UeAmbrDownlink: "1 Gbps",
			Allow4G: &no, Allow5G: &yes,
		}); err != nil {
			t.Fatalf("create profile: %s", err)
		}

		if _, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
			Name: dn, MTU: 1500, IPv4Pool: pool, DNS: "8.8.8.8",
		}); err != nil {
			t.Fatalf("create dn: %s", err)
		}

		// Legal on a 5G-only profile: the value fits the 2-octet field.
		status, _, err := createPolicy(url, client, token, &CreatePolicyParams{
			Name: "pol-" + profile, ProfileName: profile, SliceName: DefaultSliceName,
			DataNetworkName: dn, SessionAmbrUplink: ambr, SessionAmbrDownlink: ambr,
			Var5qi: 9, Arp: 1,
		})
		if err != nil || status != http.StatusCreated {
			t.Fatalf("seed policy at %s: status %d, err %v", ambr, status, err)
		}
	}

	enable4G := func(t *testing.T, profile string) int {
		t.Helper()

		status, _, err := editProfile(url, client, profile, token, &UpdateProfileParams{
			UeAmbrUplink: "1 Gbps", UeAmbrDownlink: "1 Gbps", Allow4G: &yes, Allow5G: &yes,
		})
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		return status
	}

	t.Run("a policy above the EPS ceiling blocks the transition", func(t *testing.T) {
		seed(t, "prof-20g", "dn-20g", "10.91.0.0/24", "20 Gbps")

		if status := enable4G(t, "prof-20g"); status != http.StatusBadRequest {
			t.Errorf("expected 400 (the 20 Gbps policy would be clamped to 10 Gbps on EPS), got %d", status)
		}
	})

	t.Run("a policy within the EPS ceiling permits the transition", func(t *testing.T) {
		seed(t, "prof-10g", "dn-10g", "10.92.0.0/24", "10 Gbps")

		if status := enable4G(t, "prof-10g"); status != http.StatusOK {
			t.Errorf("expected 200, got %d", status)
		}
	})
}
