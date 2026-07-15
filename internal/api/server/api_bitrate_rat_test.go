// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

// What a bitrate may be depends on the RATs the profile permits: EPS clamps
// above 10 Gbps (TS 24.008 §10.5.6.5B) and 5G cannot encode a value past 65535
// (TS 24.501 §9.11.4.14).
func TestPolicyBitrateRATLimits(t *testing.T) {
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

	cases := []struct {
		name    string
		profile string
		bitrate string
		status  int
	}{
		// The value M2 was actually about: legal, encodable, previously untypeable.
		{"1500 Mbps on a 4G+5G profile", "prof-both", "1500 Mbps", http.StatusCreated},
		{"10 Gbps on a 4G+5G profile (at the EPS ceiling)", "prof-both", "10 Gbps", http.StatusCreated},
		{"20 Gbps on a 4G+5G profile is rejected (EPS clamps)", "prof-both", "20 Gbps", http.StatusBadRequest},
		// No regression: 5G-only keeps its headroom.
		{"20 Gbps on a 5G-only profile is allowed", "prof-5gonly", "20 Gbps", http.StatusCreated},
		// 5G's value ceiling, regardless of RAT mix.
		{"100000 Mbps is rejected (5G value > 65535)", "prof-5gonly", "100000 Mbps", http.StatusBadRequest},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dn := fmt.Sprintf("dn-rat-%d", i)
			if _, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
				Name: dn, MTU: 1500, IPv4Pool: fmt.Sprintf("10.%d.0.0/24", 100+i), DNS: "8.8.8.8",
			}); err != nil {
				t.Fatalf("create dn: %s", err)
			}

			status, _, err := createPolicy(url, client, token, &CreatePolicyParams{
				Name: fmt.Sprintf("pol-rat-%d", i), ProfileName: tc.profile,
				SliceName: DefaultSliceName, DataNetworkName: dn,
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
