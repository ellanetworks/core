// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func createDataNetworkAndPolicy(database *db.Database) (string, error) {
	newDataNetwork := &db.DataNetwork{
		Name:     "not-internet",
		IPv4Pool: "1.2.3.0/24",
	}

	err := database.CreateDataNetwork(context.Background(), newDataNetwork)
	if err != nil {
		return "", err
	}

	createdNetwork, err := database.GetDataNetwork(context.Background(), newDataNetwork.Name)
	if err != nil {
		return "", err
	}

	profile := &db.Profile{
		Name:           "test-profile",
		UeAmbrUplink:   "200 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	err = database.CreateProfile(context.Background(), profile)
	if err != nil {
		return "", err
	}

	createdProfile, err := database.GetProfile(context.Background(), profile.Name)
	if err != nil {
		return "", err
	}

	slice := &db.NetworkSlice{
		Name: "test-slice",
		Sst:  1,
	}

	err = database.CreateNetworkSlice(context.Background(), slice)
	if err != nil {
		return "", err
	}

	createdSlice, err := database.GetNetworkSlice(context.Background(), slice.Name)
	if err != nil {
		return "", err
	}

	policy := &db.Policy{
		Name:                "my-policy",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       createdNetwork.ID,
		ProfileID:           createdProfile.ID,
		SliceID:             createdSlice.ID,
	}

	err = database.CreatePolicy(context.Background(), policy)
	if err != nil {
		return "", err
	}

	return createdProfile.ID, nil
}

func TestSubscribersDbEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, total, err := database.ListSubscribersPage(context.Background(), nil, 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("One or more subscribers were found in DB")
	}

	policyID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and policy: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      policyID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, total, err = database.ListSubscribersPage(context.Background(), nil, 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("One or more subscribers weren't found in DB")
	}

	retrievedSubscriber, err := database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedSubscriber.Imsi != subscriber.Imsi {
		t.Fatalf("The subscriber from the database doesn't match the subscriber that was given")
	}

	if retrievedSubscriber.SequenceNumber != subscriber.SequenceNumber {
		t.Fatalf("The sequence number from the database doesn't match the sequence number that was given")
	}

	if retrievedSubscriber.PermanentKey != subscriber.PermanentKey {
		t.Fatalf("The permanent key value from the database doesn't match the permanent key value that was given")
	}

	if retrievedSubscriber.Opc != subscriber.Opc {
		t.Fatalf("The OPC value from the database doesn't match the OPC value that was given")
	}

	newProfile := db.Profile{
		Name:           "another-profile",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "100 Mbps",
	}

	err = database.CreateProfile(context.Background(), &newProfile)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	newProfileCreated, err := database.GetProfile(context.Background(), newProfile.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	subscriber.ProfileID = newProfileCreated.ID
	if err = database.UpdateSubscriberProfile(context.Background(), subscriber); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedSubscriber, err = database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedSubscriber.ProfileID != newProfileCreated.ID {
		t.Fatalf("Profile IDs don't match: %s", retrievedSubscriber.ProfileID)
	}

	if err = database.DeleteSubscriber(context.Background(), subscriber.Imsi); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	res, total, _ = database.ListSubscribersPage(context.Background(), nil, 1, 10)

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Subscribers weren't deleted from the DB properly")
	}
}

func TestCountSubscribersInProfile(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	profileID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and policy: %s", err)
	}

	count, err := database.CountSubscribersInProfile(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 0 {
		t.Fatalf("Expected 0 subscribers in profile, but got %d", count)
	}

	subscriber1 := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "000000000001",
		PermanentKey:   "e08f6711b5319a21d550787cd263ee0a",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber1)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	newProfile := &db.Profile{
		Name:           "another-profile",
		UeAmbrUplink:   "50 Mbps",
		UeAmbrDownlink: "50 Mbps",
	}

	err = database.CreateProfile(context.Background(), newProfile)
	if err != nil {
		t.Fatalf("Couldn't Create Profile: %s", err)
	}

	newProfileCreated, err := database.GetProfile(context.Background(), newProfile.Name)
	if err != nil {
		t.Fatalf("Couldn't Retrieve Profile: %s", err)
	}

	subscriber2 := &db.Subscriber{
		Imsi:           "001010100007488",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      newProfileCreated.ID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber2)
	if err != nil {
		t.Fatalf("Couldn't Create Subscriber: %s", err)
	}

	count, err = database.CountSubscribersInProfile(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 subscriber in profile, but got %d", count)
	}

	subscriber3 := &db.Subscriber{
		Imsi:           "001010100007489",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber3)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	count, err = database.CountSubscribersInProfile(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected 2 subscribers in profile, but got %d", count)
	}
}

func TestListSubscribersFilterByDataNetwork(t *testing.T) {
	database, dnID, imsi, _ := setupLeaseTestDBWithProfile(t)
	ctx := context.Background()

	// The subscriber's profile has a policy binding this data network.
	subs, count, err := database.ListSubscribersPage(ctx, &db.SubscriberFilters{DataNetworkID: &dnID}, 1, 25)
	if err != nil {
		t.Fatalf("ListSubscribersPage(data network): %s", err)
	}

	if count != 1 || len(subs) != 1 || subs[0].Imsi != imsi {
		t.Fatalf("expected 1 entitled subscriber %s, got count=%d subs=%v", imsi, count, subs)
	}

	// A data network the profile does not reach returns none.
	other := &db.DataNetwork{Name: "other-dn", IPv4Pool: "10.5.0.0/24", DNS: "8.8.8.8", MTU: 1400}
	if err := database.CreateDataNetwork(ctx, other); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	createdOther, err := database.GetDataNetwork(ctx, other.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	subs, count, err = database.ListSubscribersPage(ctx, &db.SubscriberFilters{DataNetworkID: &createdOther.ID}, 1, 25)
	if err != nil {
		t.Fatalf("ListSubscribersPage(other data network): %s", err)
	}

	if count != 0 || len(subs) != 0 {
		t.Fatalf("expected no subscribers for unbound data network, got count=%d subs=%v", count, subs)
	}
}

func TestListSubscribersFilterBySearch(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	profileID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and policy: %s", err)
	}

	imsis := []string{"001010100007487", "001010100007488", "001010100009999"}
	for _, imsi := range imsis {
		sub := &db.Subscriber{
			Imsi:           imsi,
			SequenceNumber: "000000000001",
			PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
			Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
			ProfileID:      profileID,
		}
		if err := database.CreateSubscriber(ctx, sub); err != nil {
			t.Fatalf("Couldn't complete Create: %s", err)
		}
	}

	search := func(q string) ([]db.Subscriber, int) {
		t.Helper()

		subs, total, err := database.ListSubscribersPage(ctx, &db.SubscriberFilters{Search: &q}, 1, 25)
		if err != nil {
			t.Fatalf("ListSubscribersPage(search=%q): %s", q, err)
		}

		return subs, total
	}

	if subs, total := search("0748"); len(subs) != 2 || total != 2 {
		t.Fatalf("expected 2 matches for a shared substring, got count=%d subs=%v", total, subs)
	}

	if subs, total := search("9999"); len(subs) != 1 || total != 1 || subs[0].Imsi != "001010100009999" {
		t.Fatalf("expected the single 9999 match, got count=%d subs=%v", total, subs)
	}

	if subs, total := search("12345"); len(subs) != 0 || total != 0 {
		t.Fatalf("expected no matches, got count=%d subs=%v", total, subs)
	}

	// LIKE wildcards in the query are matched literally, not as wildcards.
	for _, q := range []string{"%", "_", `\`} {
		if subs, total := search(q); len(subs) != 0 || total != 0 {
			t.Fatalf("expected %q to match nothing, got count=%d subs=%v", q, total, subs)
		}
	}

	if subs, total := search(""); len(subs) != 3 || total != 3 {
		t.Fatalf("expected an empty search to match all, got count=%d subs=%v", total, subs)
	}

	// An out-of-range page still reports the filtered total.
	shared := "0748"

	subs, total, err := database.ListSubscribersPage(ctx, &db.SubscriberFilters{Search: &shared}, 5, 25)
	if err != nil {
		t.Fatalf("ListSubscribersPage(out of range): %s", err)
	}

	if len(subs) != 0 || total != 2 {
		t.Fatalf("expected no rows and a filtered total of 2, got count=%d subs=%v", total, subs)
	}
}

func TestListSubscribersOrdersByImsi(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	profileID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and policy: %s", err)
	}

	for _, imsi := range []string{"001010100000003", "001010100000001", "001010100000002"} {
		sub := &db.Subscriber{
			Imsi:           imsi,
			SequenceNumber: "000000000001",
			PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
			Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
			ProfileID:      profileID,
		}
		if err := database.CreateSubscriber(ctx, sub); err != nil {
			t.Fatalf("Couldn't complete Create: %s", err)
		}
	}

	first, _, err := database.ListSubscribersPage(ctx, nil, 1, 2)
	if err != nil {
		t.Fatalf("ListSubscribersPage(page 1): %s", err)
	}

	second, _, err := database.ListSubscribersPage(ctx, nil, 2, 2)
	if err != nil {
		t.Fatalf("ListSubscribersPage(page 2): %s", err)
	}

	got := []string{first[0].Imsi, first[1].Imsi, second[0].Imsi}

	want := []string{"001010100000001", "001010100000002", "001010100000003"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected pages ordered by imsi %v, got %v", want, got)
		}
	}
}
