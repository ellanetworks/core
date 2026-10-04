// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func setupMSISDNTestDB(t *testing.T) (*db.Database, string) {
	t.Helper()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	profileID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("createDataNetworkAndPolicy: %s", err)
	}

	return database, profileID
}

func newMSISDNSubscriber(imsi, msisdn, profileID string) *db.Subscriber {
	return &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
		Msisdn:         msisdn,
	}
}

func TestSubscriberMSISDNRoundTrip(t *testing.T) {
	ctx := context.Background()
	database, profileID := setupMSISDNTestDB(t)

	sub := newMSISDNSubscriber("001010000000001", "15551230001", profileID)
	if err := database.CreateSubscriber(ctx, sub); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	got, err := database.GetSubscriber(ctx, sub.Imsi)
	if err != nil {
		t.Fatalf("GetSubscriber: %s", err)
	}

	if got.Msisdn != "15551230001" {
		t.Fatalf("msisdn = %q, want %q", got.Msisdn, "15551230001")
	}

	byMSISDN, err := database.GetSubscriberByMSISDN(ctx, "15551230001")
	if err != nil {
		t.Fatalf("GetSubscriberByMSISDN: %s", err)
	}

	if byMSISDN.Imsi != sub.Imsi {
		t.Fatalf("GetSubscriberByMSISDN imsi = %q, want %q", byMSISDN.Imsi, sub.Imsi)
	}

	sub.Msisdn = "15551230002"
	if err := database.UpdateSubscriberProfile(ctx, sub); err != nil {
		t.Fatalf("UpdateSubscriberProfile: %s", err)
	}

	if _, err := database.GetSubscriberByMSISDN(ctx, "15551230001"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("old msisdn lookup: want ErrNotFound, got %v", err)
	}

	if _, err := database.GetSubscriberByMSISDN(ctx, "15551230002"); err != nil {
		t.Fatalf("new msisdn lookup: %s", err)
	}

	sub.Msisdn = ""
	if err := database.UpdateSubscriberProfile(ctx, sub); err != nil {
		t.Fatalf("UpdateSubscriberProfile clearing msisdn: %s", err)
	}

	if _, err := database.GetSubscriberByMSISDN(ctx, ""); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("empty msisdn lookup: want ErrNotFound, got %v", err)
	}
}

func TestSubscriberMSISDNIsUnique(t *testing.T) {
	ctx := context.Background()
	database, profileID := setupMSISDNTestDB(t)

	first := newMSISDNSubscriber("001010000000001", "15551230001", profileID)
	if err := database.CreateSubscriber(ctx, first); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	dup := newMSISDNSubscriber("001010000000002", "15551230001", profileID)
	if err := database.CreateSubscriber(ctx, dup); !errors.Is(err, db.ErrMSISDNInUse) {
		t.Fatalf("CreateSubscriber with taken msisdn: want ErrMSISDNInUse, got %v", err)
	}

	second := newMSISDNSubscriber("001010000000002", "", profileID)
	if err := database.CreateSubscriber(ctx, second); err != nil {
		t.Fatalf("CreateSubscriber without msisdn: %s", err)
	}

	third := newMSISDNSubscriber("001010000000003", "", profileID)
	if err := database.CreateSubscriber(ctx, third); err != nil {
		t.Fatalf("second CreateSubscriber without msisdn: %s", err)
	}

	second.Msisdn = "15551230001"
	if err := database.UpdateSubscriberProfile(ctx, second); !errors.Is(err, db.ErrMSISDNInUse) {
		t.Fatalf("UpdateSubscriberProfile with taken msisdn: want ErrMSISDNInUse, got %v", err)
	}

	if err := database.UpdateSubscriberProfile(ctx, first); err != nil {
		t.Fatalf("UpdateSubscriberProfile keeping own msisdn: %s", err)
	}

	existing := newMSISDNSubscriber("001010000000001", "15551230009", profileID)
	if err := database.CreateSubscriber(ctx, existing); !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("CreateSubscriber with taken imsi: want ErrAlreadyExists, got %v", err)
	}
}

func TestListSubscribersSearchMatchesMSISDN(t *testing.T) {
	ctx := context.Background()
	database, profileID := setupMSISDNTestDB(t)

	for _, sub := range []*db.Subscriber{
		newMSISDNSubscriber("001010000000001", "15551230001", profileID),
		newMSISDNSubscriber("001010000000002", "4930998877", profileID),
	} {
		if err := database.CreateSubscriber(ctx, sub); err != nil {
			t.Fatalf("CreateSubscriber: %s", err)
		}
	}

	search := "99887"

	subs, total, err := database.ListSubscribersPage(ctx, &db.SubscriberFilters{Search: &search}, 1, 10)
	if err != nil {
		t.Fatalf("ListSubscribersPage: %s", err)
	}

	if total != 1 || len(subs) != 1 || subs[0].Imsi != "001010000000002" || subs[0].Msisdn != "4930998877" {
		t.Fatalf("search by msisdn: total=%d subs=%+v", total, subs)
	}
}

func TestIsValidMSISDN(t *testing.T) {
	cases := map[string]bool{
		"":                 false,
		"1":                true,
		"15551230001":      true,
		"123456789012345":  true,
		"1234567890123456": false,
		"+15551230001":     false,
		"05551230001":      false,
		"1555123000a":      false,
	}

	for msisdn, want := range cases {
		if got := db.IsValidMSISDN(msisdn); got != want {
			t.Errorf("IsValidMSISDN(%q) = %v, want %v", msisdn, got, want)
		}
	}
}

func TestIMSIsWithMSISDN(t *testing.T) {
	ctx := context.Background()
	database, profileID := setupMSISDNTestDB(t)

	for imsi, msisdn := range map[string]string{"001010000000001": "15551230001", "001010000000002": ""} {
		if err := database.CreateSubscriber(ctx, newMSISDNSubscriber(imsi, msisdn, profileID)); err != nil {
			t.Fatalf("CreateSubscriber: %s", err)
		}
	}

	found, err := database.IMSIsWithMSISDN(ctx, []string{"001010000000001", "001010000000002", "001019999999999"})
	if err != nil {
		t.Fatalf("IMSIsWithMSISDN: %s", err)
	}

	if len(found) != 1 || !found["001010000000001"] {
		t.Fatalf("found = %v, want only the subscriber with an MSISDN", found)
	}
}
