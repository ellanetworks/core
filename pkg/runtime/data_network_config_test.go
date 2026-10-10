// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"net/netip"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDataNetworkConfigCarriesPCSCFAddressesOnlyOnIMS(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	profile := &db.Profile{Name: "voice", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}
	if err := database.CreateProfile(ctx, profile); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	createdProfile, err := database.GetProfile(ctx, profile.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	slice := &db.NetworkSlice{Name: "embb", Sst: 1}
	if err := database.CreateNetworkSlice(ctx, slice); err != nil {
		t.Fatalf("CreateNetworkSlice: %s", err)
	}

	createdSlice, err := database.GetNetworkSlice(ctx, slice.Name)
	if err != nil {
		t.Fatalf("GetNetworkSlice: %s", err)
	}

	for i, name := range []string{"ims", "enterprise"} {
		dn := &db.DataNetwork{Name: name, IPv4Pool: []string{"10.46.0.0/16", "10.45.0.0/16"}[i], DNS: "8.8.8.8", MTU: 1400}
		if err := database.CreateDataNetwork(ctx, dn); err != nil {
			t.Fatalf("CreateDataNetwork %s: %s", name, err)
		}

		created, err := database.GetDataNetwork(ctx, name)
		if err != nil {
			t.Fatalf("GetDataNetwork %s: %s", name, err)
		}

		policy := &db.Policy{Name: name, DataNetworkID: created.ID, ProfileID: createdProfile.ID, SliceID: createdSlice.ID, Var5qi: 9, Arp: 1, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps"}
		if err := database.CreatePolicy(ctx, policy); err != nil {
			t.Fatalf("CreatePolicy %s: %s", name, err)
		}
	}

	imsi := "001010123456789"
	if err := database.CreateSubscriber(ctx, &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      createdProfile.ID,
	}); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	want := []netip.Addr{netip.MustParseAddr("2001:db8::5"), netip.MustParseAddr("10.0.0.5")}
	if err := database.ReplacePCSCFAddresses(ctx, want); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	store := &smfDBAdapter{db: database}

	ims, err := store.ResolveDNN(ctx, "ims")
	if err != nil {
		t.Fatalf("ResolveDNN ims: %s", err)
	}

	imsConfig, err := ims.Config(ctx)
	if err != nil {
		t.Fatalf("Config ims: %s", err)
	}

	if got := imsConfig.PCSCF; !slices.Equal(got, want) {
		t.Fatalf("ims P-CSCF = %v, want %v", got, want)
	}

	enterprise, err := store.ResolveDNN(ctx, "enterprise")
	if err != nil {
		t.Fatalf("ResolveDNN enterprise: %s", err)
	}

	enterpriseConfig, err := enterprise.Config(ctx)
	if err != nil {
		t.Fatalf("Config enterprise: %s", err)
	}

	if got := enterpriseConfig.PCSCF; len(got) != 0 {
		t.Fatalf("enterprise P-CSCF = %v, want none", got)
	}
}
