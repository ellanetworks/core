// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestListSubscribers_RegisteredOnAnotherNode(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL
	ctx := context.Background()

	const (
		on4GElsewhere   = "001010100000071"
		onBothElsewhere = "001010100000072"
		purgedElsewhere = "001010100000073"
		unregistered    = "001010100000075"
	)

	mustCreate := func(name string, sc int, err error) {
		t.Helper()

		if err != nil || sc != http.StatusCreated {
			t.Fatalf("%s: %d %v", name, sc, err)
		}
	}

	sc, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{Name: "cluster-dn", IPv4Pool: IPv4Pool, DNS: DNS, MTU: MTU})
	mustCreate("createDataNetwork", sc, err)

	sc, _, err = createSlice(url, client, token, &CreateSliceParams{Name: "cluster-slice", Sst: 1})
	mustCreate("createSlice", sc, err)

	sc, _, err = createProfile(url, client, token, &CreateProfileParams{Name: TestProfileName, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"})
	mustCreate("createProfile", sc, err)

	sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
		Name: "cluster-policy", ProfileName: TestProfileName, SliceName: "cluster-slice",
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkName: "cluster-dn",
	})
	mustCreate("createPolicy", sc, err)

	for _, imsi := range []string{on4GElsewhere, onBothElsewhere, purgedElsewhere, unregistered} {
		sc, r, err := createSubscriber(url, client, token, &CreateSubscriberParams{Imsi: imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: TestProfileName})
		if err != nil || sc != http.StatusCreated {
			t.Fatalf("createSubscriber %s: %d %v %+v", imsi, sc, err, r)
		}
	}

	register := func(imsi, regType, nodeID string) {
		t.Helper()

		if _, err := env.DB.RegisterUE(ctx, imsi, regType, nodeID, ""); err != nil {
			t.Fatalf("RegisterUE %s %s: %s", imsi, regType, err)
		}
	}

	register(on4GElsewhere, db.UERegistrationTypeMME, "3")
	register(onBothElsewhere, db.UERegistrationTypeMME, "3")
	register(onBothElsewhere, db.UERegistrationTypeAMF3GPPAccess, "2")
	register(purgedElsewhere, db.UERegistrationTypeMME, "3")

	if err := env.DB.PurgeUERegistration(ctx, purgedElsewhere, db.UERegistrationTypeMME, "3"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	code, resp, err := listSubscribers(url, client, token, 1, 25)
	if err != nil || code != http.StatusOK {
		t.Fatalf("listSubscribers: %d %v", code, err)
	}

	got := make(map[string]ListSubscriberStatus, len(resp.Result.Items))
	for _, item := range resp.Result.Items {
		got[item.Imsi] = item.Status
	}

	for _, tc := range []struct {
		imsi        string
		registered  bool
		wantSystems []string
	}{
		{imsi: on4GElsewhere, registered: true, wantSystems: []string{"4G"}},
		{imsi: onBothElsewhere, registered: true, wantSystems: []string{"5G", "4G"}},
		{imsi: purgedElsewhere},
		{imsi: unregistered},
	} {
		status, ok := got[tc.imsi]
		if !ok {
			t.Fatalf("subscriber %s missing from list", tc.imsi)
		}

		if status.Registered != tc.registered {
			t.Errorf("%s: registered = %v, want %v", tc.imsi, status.Registered, tc.registered)
		}

		if !slices.Equal(status.Systems, tc.wantSystems) {
			t.Errorf("%s: systems = %v, want %v", tc.imsi, status.Systems, tc.wantSystems)
		}

		if status.ConnectionState != "" || status.NumSessions != 0 {
			t.Errorf("%s: per-node fields = {%q, %d}, want empty", tc.imsi, status.ConnectionState, status.NumSessions)
		}
	}
}
