// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

func TestSubscriberDnnPrefersTheDefaultPolicy(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateProfile(ctx, &db.Profile{Name: "voice", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	profile, err := database.GetProfile(ctx, "voice")
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	if err := database.CreateNetworkSlice(ctx, &db.NetworkSlice{Name: "embb", Sst: 1}); err != nil {
		t.Fatalf("CreateNetworkSlice: %s", err)
	}

	slice, err := database.GetNetworkSlice(ctx, "embb")
	if err != nil {
		t.Fatalf("GetNetworkSlice: %s", err)
	}

	for i, name := range []string{"enterprise", "ims"} {
		if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: name, IPv4Pool: []string{"10.45.0.0/16", "10.46.0.0/16"}[i], DNS: "8.8.8.8", MTU: 1400}); err != nil {
			t.Fatalf("CreateDataNetwork %s: %s", name, err)
		}

		dn, err := database.GetDataNetwork(ctx, name)
		if err != nil {
			t.Fatalf("GetDataNetwork %s: %s", name, err)
		}

		if err := database.CreatePolicy(ctx, &db.Policy{Name: name, DataNetworkID: dn.ID, ProfileID: profile.ID, SliceID: slice.ID, Var5qi: 9, Arp: 1, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps"}); err != nil {
			t.Fatalf("CreatePolicy %s: %s", name, err)
		}
	}

	imsi := "001010123456789"
	if err := database.CreateSubscriber(ctx, &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profile.ID,
	}); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	a := amf.New(nil, nil, nil)
	a.DBInstance = database

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatal(err)
	}

	snssai := &models.Snssai{Sst: 1}

	for _, want := range []string{"ims", "enterprise"} {
		if err := database.SetDefaultPolicy(ctx, profile.ID, want); err != nil {
			t.Fatalf("SetDefaultPolicy %s: %s", want, err)
		}

		got, err := a.SubscriberDnn(ctx, supi, snssai)
		if err != nil {
			t.Fatalf("SubscriberDnn: %s", err)
		}

		if got != want {
			t.Fatalf("SubscriberDnn with default policy %s = %q", want, got)
		}
	}
}
