// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

// UE-AMBR rides S1AP (BitRate ::= INTEGER (0..10000000000), TS 36.413) and NGAP
// (BitRate ::= INTEGER (0..4000000000000, ...), TS 38.413), so its bounds are
// the ASN.1 ranges of those IEs and differ per RAT. It never travels in NAS, so
// the 65535 Session-AMBR value bound does not apply to it.
func TestProfileUeAmbrRATLimits(t *testing.T) {
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

	cases := []struct {
		name         string
		allow4G      bool
		bitrate      string
		createStatus int
	}{
		// Not a NAS IE: 100000 Mbps is unencodable as a Session-AMBR (value >
		// 65535) yet perfectly legal as a 5G UE-AMBR at 100 Gbps.
		{"100000 Mbps on a 5G-only profile is allowed", false, "100000 Mbps", http.StatusCreated},
		{"4 Tbps on a 5G-only profile (at the NGAP ceiling)", false, "4000 Gbps", http.StatusCreated},
		{"5 Tbps on a 5G-only profile is rejected", false, "5000 Gbps", http.StatusBadRequest},
		{"10 Gbps on a 4G profile (at the S1AP ceiling)", true, "10 Gbps", http.StatusCreated},
		// Above the S1AP range the InitialContextSetupRequest fails to encode
		// entirely, so the UE never attaches.
		{"20 Gbps on a 4G profile is rejected", true, "20 Gbps", http.StatusBadRequest},
		{"100 Gbps on a 4G profile is rejected", true, "100 Gbps", http.StatusBadRequest},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allow4G := no
			if tc.allow4G {
				allow4G = yes
			}

			status, _, err := createProfile(url, client, token, &CreateProfileParams{
				Name: fmt.Sprintf("prof-ue-%d", i), UeAmbrUplink: tc.bitrate, UeAmbrDownlink: tc.bitrate,
				Allow4G: &allow4G, Allow5G: &yes,
			})
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			if status != tc.createStatus {
				t.Errorf("create with allow_4g=%v @ %q: expected %d, got %d", tc.allow4G, tc.bitrate, tc.createStatus, status)
			}
		})
	}
}

// An edit answers to the same bounds, and the RATs named in the request decide
// which apply: turning 4G on drops the UE-AMBR ceiling from 4 Tbps to 10 Gbps.
func TestUpdateProfileUeAmbrRATLimits(t *testing.T) {
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
		Name: "prof-edit", UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps",
		Allow4G: &no, Allow5G: &yes,
	}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	edit := func(t *testing.T, allow4G bool, bitrate string, want int) {
		t.Helper()

		a4 := no
		if allow4G {
			a4 = yes
		}

		status, _, err := editProfile(url, client, "prof-edit", token, &UpdateProfileParams{
			UeAmbrUplink: bitrate, UeAmbrDownlink: bitrate, Allow4G: &a4, Allow5G: &yes,
		})
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if status != want {
			t.Errorf("edit allow_4g=%v @ %q: expected %d, got %d", allow4G, bitrate, want, status)
		}
	}

	t.Run("raising a 5G-only profile to 100 Gbps is allowed", func(t *testing.T) {
		edit(t, false, "100 Gbps", http.StatusOK)
	})

	// The RATs in the request decide, not the ones the profile had.
	t.Run("enabling 4G on a 100 Gbps profile is rejected", func(t *testing.T) {
		edit(t, true, "100 Gbps", http.StatusBadRequest)
	})

	t.Run("enabling 4G together with a 10 Gbps UE-AMBR is allowed", func(t *testing.T) {
		edit(t, true, "10 Gbps", http.StatusOK)
	})
}
