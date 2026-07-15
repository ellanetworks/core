// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

// Both radio paths carry kbps-scaled bitrates, so a sub-Mbps rate limit must be
// configurable. "bps" stays rejected: the SMF truncates it to kbps.
func TestPolicyBitrateUnits(t *testing.T) {
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

	if _, _, err := createProfile(url, client, token, &CreateProfileParams{
		Name: "prof-units", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps",
	}); err != nil {
		t.Fatalf("create profile: %s", err)
	}

	cases := []struct {
		name    string
		dn      string
		bitrate string
		status  int
	}{
		{"Kbps is accepted", "internet", "500 Kbps", http.StatusCreated},
		{"Mbps is accepted", "dn-mbps", "100 Mbps", http.StatusCreated},
		{"bps is rejected (SMF truncates it)", "dn-bps", "500000 bps", http.StatusBadRequest},
		{"unknown unit is rejected", "dn-bogus", "5 Furlongs", http.StatusBadRequest},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dn != "internet" {
				if _, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
					Name: tc.dn, MTU: 1500, IPv4Pool: "10.9" + string(rune('0'+i)) + ".0.0/24", DNS: "8.8.8.8",
				}); err != nil {
					t.Fatalf("create dn: %s", err)
				}
			}

			status, _, err := createPolicy(url, client, token, &CreatePolicyParams{
				Name: "pol-" + tc.dn, ProfileName: "prof-units", SliceName: DefaultSliceName,
				DataNetworkName: tc.dn, SessionAmbrUplink: tc.bitrate,
				SessionAmbrDownlink: tc.bitrate, Var5qi: 9, Arp: 1,
			})
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			if status != tc.status {
				t.Errorf("%q: expected %d, got %d", tc.bitrate, tc.status, status)
			}
		})
	}
}
