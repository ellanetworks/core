// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

func TestCreateAssociationDecision(t *testing.T) {
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

	p := pcf.New(database, zap.NewNop())

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatal(err)
	}

	subscribed := smf.SubscribedQoS{Var5qi: 5, Arp: 3, SessionAMBR: models.Ambr{Uplink: models.MustParseBitRate("10 Mbps"), Downlink: models.MustParseBitRate("20 Mbps")}}

	ims, err := database.GetPolicy(ctx, "ims")
	if err != nil {
		t.Fatalf("GetPolicy ims: %s", err)
	}

	d, err := p.CreateAssociation(ctx, "ims-session", smf.PolicyContext{Supi: supi, Dnn: "ims", Snssai: models.Snssai{Sst: 1}, Subscribed: subscribed})
	if err != nil {
		t.Fatalf("CreateAssociation: %s", err)
	}

	if d.PolicyID != ims.ID || d.Var5qi != 5 || d.Arp != 3 || !d.SessionAMBR.Downlink.Equal(subscribed.SessionAMBR.Downlink) {
		t.Fatalf("decision = %+v, want policy %s with the subscribed QoS", d, ims.ID)
	}

	if _, err := p.CreateAssociation(ctx, "other", smf.PolicyContext{Supi: supi, Dnn: "missing", Snssai: models.Snssai{Sst: 1}}); !errors.Is(err, smf.ErrDNNNotInSlice) {
		t.Fatalf("CreateAssociation on an unsubscribed DNN = %v, want ErrDNNNotInSlice", err)
	}

	subscribed.Var5qi = 6

	updated, err := p.UpdateAssociation(ctx, "ims-session", subscribed)
	if err != nil || updated.Var5qi != 6 || updated.Revision <= d.Revision {
		t.Fatalf("UpdateAssociation = %+v, %v; want 5QI 6 at a later revision", updated, err)
	}
}
