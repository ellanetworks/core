// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

// setupFramedRoutesTestDB creates a data network and a subscriber, returning the
// database, the data network ID, and the subscriber IMSI.
func setupFramedRoutesTestDB(t *testing.T) (*db.Database, string, string) {
	t.Helper()

	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	dn := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), dn); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	createdDN, err := database.GetDataNetwork(context.Background(), dn.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	profile := &db.Profile{Name: "test-profile", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}
	if err := database.CreateProfile(context.Background(), profile); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	createdProfile, err := database.GetProfile(context.Background(), profile.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	imsi := "001010123456789"

	sub := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      createdProfile.ID,
	}

	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	return database, createdDN.ID, imsi
}

func createSubscriber(t *testing.T, database *db.Database, imsi string) {
	t.Helper()

	profile := &db.Profile{Name: "profile-" + imsi, UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}
	if err := database.CreateProfile(context.Background(), profile); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	created, err := database.GetProfile(context.Background(), profile.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	sub := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      created.ID,
	}
	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}
}

func prefixes(t *testing.T, cidrs ...string) []netip.Prefix {
	t.Helper()

	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			t.Fatalf("ParsePrefix(%q): %s", c, err)
		}

		out = append(out, p)
	}

	return out
}

func framedPrefixSet(routes []db.SubscriberFramedRoute) map[string]struct{} {
	set := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		set[r.Prefix] = struct{}{}
	}

	return set
}

func TestFramedRoutes_ReplaceAndList(t *testing.T) {
	database, dnID, imsi := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	if err := database.ReplaceFramedRoutes(ctx, imsi, dnID, prefixes(t, "192.168.10.0/24", "2001:db8:1::/48")); err != nil {
		t.Fatalf("ReplaceFramedRoutes: %s", err)
	}

	got, err := database.ListFramedRoutesBySubscriberDataNetwork(ctx, imsi, dnID)
	if err != nil {
		t.Fatalf("ListFramedRoutesBySubscriberDataNetwork: %s", err)
	}

	set := framedPrefixSet(got)
	if len(set) != 2 {
		t.Fatalf("expected 2 framed routes, got %d: %v", len(set), got)
	}

	for _, want := range []string{"192.168.10.0/24", "2001:db8:1::/48"} {
		if _, ok := set[want]; !ok {
			t.Fatalf("expected framed route %s, got %v", want, got)
		}
	}
}

func TestFramedRoutes_ReplaceIsAWholeSetSwap(t *testing.T) {
	database, dnID, imsi := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	if err := database.ReplaceFramedRoutes(ctx, imsi, dnID, prefixes(t, "192.168.10.0/24", "192.168.11.0/24")); err != nil {
		t.Fatalf("first ReplaceFramedRoutes: %s", err)
	}

	if err := database.ReplaceFramedRoutes(ctx, imsi, dnID, prefixes(t, "192.168.11.0/24", "192.168.12.0/24")); err != nil {
		t.Fatalf("second ReplaceFramedRoutes: %s", err)
	}

	got, err := database.ListFramedRoutesBySubscriberDataNetwork(ctx, imsi, dnID)
	if err != nil {
		t.Fatalf("ListFramedRoutesBySubscriberDataNetwork: %s", err)
	}

	set := framedPrefixSet(got)
	if len(set) != 2 {
		t.Fatalf("expected 2 framed routes after replace, got %d: %v", len(set), got)
	}

	if _, ok := set["192.168.10.0/24"]; ok {
		t.Fatalf("expected 192.168.10.0/24 to be removed by replace, got %v", got)
	}

	for _, want := range []string{"192.168.11.0/24", "192.168.12.0/24"} {
		if _, ok := set[want]; !ok {
			t.Fatalf("expected framed route %s after replace, got %v", want, got)
		}
	}
}

func TestFramedRoutes_Delete(t *testing.T) {
	database, dnID, imsi := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	if err := database.ReplaceFramedRoutes(ctx, imsi, dnID, prefixes(t, "192.168.10.0/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes: %s", err)
	}

	if err := database.DeleteFramedRoutes(ctx, imsi, dnID); err != nil {
		t.Fatalf("DeleteFramedRoutes: %s", err)
	}

	got, err := database.ListFramedRoutesBySubscriberDataNetwork(ctx, imsi, dnID)
	if err != nil {
		t.Fatalf("ListFramedRoutesBySubscriberDataNetwork: %s", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected no framed routes after delete, got %v", got)
	}
}

func TestFramedRoutes_PrefixNormalizedBeforeStorage(t *testing.T) {
	database, dnID, imsi := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	// Host bits set; storage must hold the masked network form.
	if err := database.ReplaceFramedRoutes(ctx, imsi, dnID, prefixes(t, "192.168.10.5/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes: %s", err)
	}

	got, err := database.ListFramedRoutesBySubscriberDataNetwork(ctx, imsi, dnID)
	if err != nil {
		t.Fatalf("ListFramedRoutesBySubscriberDataNetwork: %s", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 framed route, got %d: %v", len(got), got)
	}

	if got[0].Prefix != "192.168.10.0/24" {
		t.Fatalf("expected stored prefix 192.168.10.0/24, got %s", got[0].Prefix)
	}
}

func TestFramedRoutes_PrefixGloballyUnique(t *testing.T) {
	database, dnID, imsiA := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	imsiB := "001010123456790"
	createSubscriber(t, database, imsiB)

	if err := database.ReplaceFramedRoutes(ctx, imsiA, dnID, prefixes(t, "192.168.10.0/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes(A): %s", err)
	}

	err := database.ReplaceFramedRoutes(ctx, imsiB, dnID, prefixes(t, "192.168.10.0/24"))
	if !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists for a prefix owned by another subscriber, got %v", err)
	}

	// A's route survives B's rejected write.
	got, err := database.ListFramedRoutesBySubscriberDataNetwork(ctx, imsiA, dnID)
	if err != nil {
		t.Fatalf("ListFramedRoutesBySubscriberDataNetwork(A): %s", err)
	}

	if len(got) != 1 || got[0].Prefix != "192.168.10.0/24" {
		t.Fatalf("expected A to keep 192.168.10.0/24, got %v", got)
	}
}

func TestFramedRoutes_ListByDataNetwork(t *testing.T) {
	database, dnID, imsiA := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	imsiB := "001010123456790"
	createSubscriber(t, database, imsiB)

	if err := database.ReplaceFramedRoutes(ctx, imsiA, dnID, prefixes(t, "192.168.10.0/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes(A): %s", err)
	}

	if err := database.ReplaceFramedRoutes(ctx, imsiB, dnID, prefixes(t, "192.168.20.0/24", "192.168.21.0/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes(B): %s", err)
	}

	got, err := database.ListFramedRoutesByDataNetwork(ctx, dnID)
	if err != nil {
		t.Fatalf("ListFramedRoutesByDataNetwork: %s", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 framed routes on the data network, got %d: %v", len(got), got)
	}

	byIMSI := map[string]int{}
	for _, r := range got {
		byIMSI[r.Imsi]++
	}

	if byIMSI[imsiA] != 1 || byIMSI[imsiB] != 2 {
		t.Fatalf("expected A=1 B=2 framed routes, got %v", byIMSI)
	}
}

func TestFramedRoutes_CascadeOnSubscriberDelete(t *testing.T) {
	database, dnID, imsi := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	if err := database.ReplaceFramedRoutes(ctx, imsi, dnID, prefixes(t, "192.168.10.0/24", "2001:db8:1::/48")); err != nil {
		t.Fatalf("ReplaceFramedRoutes: %s", err)
	}

	if err := database.DeleteSubscriber(ctx, imsi); err != nil {
		t.Fatalf("DeleteSubscriber: %s", err)
	}

	all, err := database.ListAllFramedRoutes(ctx)
	if err != nil {
		t.Fatalf("ListAllFramedRoutes: %s", err)
	}

	if len(all) != 0 {
		t.Fatalf("expected subscriber delete to cascade framed routes, got %v", all)
	}
}

func TestFramedRoutes_DataNetworkDeleteRestricted(t *testing.T) {
	database, dnID, imsi := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	if err := database.ReplaceFramedRoutes(ctx, imsi, dnID, prefixes(t, "192.168.10.0/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes: %s", err)
	}

	// ON DELETE RESTRICT: a data network referenced by a framed route cannot be
	// deleted until the route is removed.
	if err := database.DeleteDataNetwork(ctx, "test-dnn"); err == nil {
		t.Fatalf("expected DeleteDataNetwork to be rejected while a framed route references it")
	}

	if err := database.DeleteFramedRoutes(ctx, imsi, dnID); err != nil {
		t.Fatalf("DeleteFramedRoutes: %s", err)
	}

	if err := database.DeleteDataNetwork(ctx, "test-dnn"); err != nil {
		t.Fatalf("expected DeleteDataNetwork to succeed after the framed route is removed, got %s", err)
	}
}

func TestFramedRoutes_PairScopedAcrossDataNetworks(t *testing.T) {
	database, dn1ID, imsi := setupFramedRoutesTestDB(t)
	ctx := context.Background()

	dn2 := &db.DataNetwork{Name: "test-dnn-2", IPv4Pool: "10.2.0.0/24"}
	if err := database.CreateDataNetwork(ctx, dn2); err != nil {
		t.Fatalf("CreateDataNetwork(2): %s", err)
	}

	created2, err := database.GetDataNetwork(ctx, dn2.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork(2): %s", err)
	}

	dn2ID := created2.ID

	if err := database.ReplaceFramedRoutes(ctx, imsi, dn1ID, prefixes(t, "192.168.1.0/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes(dn1): %s", err)
	}

	if err := database.ReplaceFramedRoutes(ctx, imsi, dn2ID, prefixes(t, "192.168.2.0/24")); err != nil {
		t.Fatalf("ReplaceFramedRoutes(dn2): %s", err)
	}

	got1, err := database.ListFramedRoutesBySubscriberDataNetwork(ctx, imsi, dn1ID)
	if err != nil {
		t.Fatalf("ListFramedRoutesBySubscriberDataNetwork(dn1): %s", err)
	}

	if len(got1) != 1 || got1[0].Prefix != "192.168.1.0/24" {
		t.Fatalf("expected only the dn1 route, got %v", got1)
	}

	got2, err := database.ListFramedRoutesBySubscriberDataNetwork(ctx, imsi, dn2ID)
	if err != nil {
		t.Fatalf("ListFramedRoutesBySubscriberDataNetwork(dn2): %s", err)
	}

	if len(got2) != 1 || got2[0].Prefix != "192.168.2.0/24" {
		t.Fatalf("expected only the dn2 route, got %v", got2)
	}

	byDN1, err := database.ListFramedRoutesByDataNetwork(ctx, dn1ID)
	if err != nil {
		t.Fatalf("ListFramedRoutesByDataNetwork(dn1): %s", err)
	}

	if len(byDN1) != 1 || byDN1[0].DataNetworkID != dn1ID {
		t.Fatalf("expected only dn1 routes from the data-network listing, got %v", byDN1)
	}
}
