// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

const subscriptionIMSI = "001010100007487"

func subscriptionFixture(t *testing.T) (*db.Database, *udm.Subscriptions, string) {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %s", err)
	}

	if err := database.CreateSubscriber(ctx, &db.Subscriber{
		Imsi:           subscriptionIMSI,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profile.ID,
	}); err != nil {
		t.Fatalf("create subscriber: %s", err)
	}

	if err := database.CreateNetworkSlice(ctx, &db.NetworkSlice{Name: "second-slice", Sst: 2}); err != nil {
		t.Fatalf("create slice: %s", err)
	}

	second, err := database.GetNetworkSlice(ctx, "second-slice")
	if err != nil {
		t.Fatalf("get slice: %s", err)
	}

	internet, err := database.GetDataNetwork(ctx, "internet")
	if err != nil {
		t.Fatalf("get data network: %s", err)
	}

	if err := database.CreatePolicy(ctx, &db.Policy{
		Name: "internet-on-second", ProfileID: profile.ID, SliceID: second.ID, DataNetworkID: internet.ID,
		Var5qi: 8, Arp: 2, SessionAmbrUplink: "10 Mbps", SessionAmbrDownlink: "10 Mbps",
	}); err != nil {
		t.Fatalf("create policy: %s", err)
	}

	return database, udm.NewSubscriptions(database, nil), profile.ID
}

func TestSessionManagementSelectsTheEPSSlice(t *testing.T) {
	ctx := context.Background()
	database, subs, profileID := subscriptionFixture(t)

	sm, err := subs.SessionManagement(ctx, subscriptionIMSI)
	if err != nil {
		t.Fatalf("SessionManagement: %s", err)
	}

	c, ok := sm.ForAPN("internet")
	if !ok || c.Snssai.Sst != db.InitialSliceSst {
		t.Fatalf("ForAPN(internet) = %+v, %t; want the default policy's slice", c, ok)
	}

	if err := database.SetDefaultPolicy(ctx, profileID, "internet-on-second"); err != nil {
		t.Fatalf("set default: %s", err)
	}

	if sm, err = subs.SessionManagement(ctx, subscriptionIMSI); err != nil {
		t.Fatalf("SessionManagement: %s", err)
	}

	if c, ok = sm.ForAPN("internet"); !ok || c.Snssai.Sst != 2 {
		t.Fatalf("ForAPN(internet) = %+v, %t; want the second slice", c, ok)
	}

	if d, ok := sm.DefaultAPN(); !ok || d.DNN != "internet" || d.Snssai.Sst != 2 {
		t.Fatalf("DefaultAPN() = %+v, %t; want internet on the second slice", d, ok)
	}

	if dnn, ok := sm.DefaultDNN(models.Snssai{Sst: 2}); !ok || dnn != "internet" {
		t.Fatalf("DefaultDNN(sst 2) = %q, %t; want internet", dnn, ok)
	}

	if _, ok := sm.ForAPN("nonexistent-apn"); ok {
		t.Fatal("ForAPN(nonexistent-apn) found a DNN configuration")
	}

	if _, ok := sm.DefaultDNN(models.Snssai{Sst: 3}); ok {
		t.Fatal("DefaultDNN on an unsubscribed slice found a DNN")
	}
}

func TestDefaultDNNPrefersTheDefaultPolicyOnASharedSlice(t *testing.T) {
	ctx := context.Background()
	database, subs, profileID := subscriptionFixture(t)

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: "ims", IPv4Pool: "10.46.0.0/16", DNS: "8.8.8.8", MTU: 1400}); err != nil {
		t.Fatalf("create data network: %s", err)
	}

	ims, err := database.GetDataNetwork(ctx, "ims")
	if err != nil {
		t.Fatalf("get data network: %s", err)
	}

	slice, err := database.GetNetworkSlice(ctx, db.InitialSliceName)
	if err != nil {
		t.Fatalf("get slice: %s", err)
	}

	if err := database.CreatePolicy(ctx, &db.Policy{
		Name: "ims", ProfileID: profileID, SliceID: slice.ID, DataNetworkID: ims.ID,
		Var5qi: 5, Arp: 1, SessionAmbrUplink: "10 Mbps", SessionAmbrDownlink: "10 Mbps",
	}); err != nil {
		t.Fatalf("create policy: %s", err)
	}

	for _, tc := range []struct{ policy, dnn string }{{"ims", "ims"}, {db.InitialPolicyName, db.InitialDataNetworkName}} {
		if err := database.SetDefaultPolicy(ctx, profileID, tc.policy); err != nil {
			t.Fatalf("set default %s: %s", tc.policy, err)
		}

		sm, err := subs.SessionManagement(ctx, subscriptionIMSI)
		if err != nil {
			t.Fatalf("SessionManagement: %s", err)
		}

		if dnn, ok := sm.DefaultDNN(models.Snssai{Sst: db.InitialSliceSst}); !ok || dnn != tc.dnn {
			t.Fatalf("DefaultDNN with default policy %s = %q, %t; want %s", tc.policy, dnn, ok, tc.dnn)
		}
	}
}

func TestAccessAndMobilitySubscription(t *testing.T) {
	ctx := context.Background()
	_, subs, _ := subscriptionFixture(t)

	am, err := subs.AccessAndMobility(ctx, subscriptionIMSI)
	if err != nil {
		t.Fatalf("AccessAndMobility: %s", err)
	}

	ssts := make([]int32, 0, len(am.SubscribedSNSSAIs))
	for _, s := range am.SubscribedSNSSAIs {
		ssts = append(ssts, s.Sst)
	}

	slices.Sort(ssts)

	if !slices.Equal(ssts, []int32{db.InitialSliceSst, 2}) || !am.Allow4G || !am.Allow5G || am.UEAMBR.Uplink.IsZero() {
		t.Fatalf("access and mobility subscription = %+v", am)
	}
}

func TestSessionManagementRejectsAnUnusableSessionAMBR(t *testing.T) {
	ctx := context.Background()
	database, subs, _ := subscriptionFixture(t)

	broken, err := database.GetPolicy(ctx, "internet-on-second")
	if err != nil {
		t.Fatalf("get policy: %s", err)
	}

	broken.SessionAmbrUplink = "not a bitrate"
	if err := database.UpdatePolicy(ctx, broken); err != nil {
		t.Fatalf("update policy: %s", err)
	}

	sm, err := subs.SessionManagement(ctx, subscriptionIMSI)
	if err == nil || !strings.Contains(err.Error(), broken.ID) {
		t.Fatalf("SessionManagement = %+v, %v; want an error naming policy %s", sm, err, broken.ID)
	}
}

func TestSessionManagementOfADeletedSubscriber(t *testing.T) {
	ctx := context.Background()
	database, subs, _ := subscriptionFixture(t)

	if err := database.DeleteSubscriber(ctx, subscriptionIMSI); err != nil {
		t.Fatalf("delete subscriber: %s", err)
	}

	if _, err := subs.SessionManagement(ctx, subscriptionIMSI); !errors.Is(err, udm.ErrSubscriberUnknown) {
		t.Fatalf("SessionManagement: %v, want ErrSubscriberUnknown", err)
	}
}
