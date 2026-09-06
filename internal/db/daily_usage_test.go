// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func createDataNetworkPolicyAndSubscriber(database *db.Database, imsi string) (string, error) {
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

	subscriber := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      createdProfile.ID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		return "", err
	}

	return createdProfile.ID, nil
}

func TestGetUsagePerDay_1Sub(t *testing.T) {
	database := setupTestDB(t)

	imsi := "001010100007487"

	_, err := createDataNetworkPolicyAndSubscriber(database, imsi)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	dailyUsages, err := database.GetUsagePerDay(context.Background(), "", startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage for period: %s", err)
	}

	if len(dailyUsages) != 1 {
		t.Fatalf("Expected 1 daily usage entry, but got %d", len(dailyUsages))
	}

	if dailyUsages[0].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", dailyUsages[0].BytesUplink)
	}

	if dailyUsages[0].BytesDownlink != 2000 {
		t.Fatalf("Expected 2000 downlink bytes, but got %d", dailyUsages[0].BytesDownlink)
	}

	expectedEpochDay := db.DaysSinceEpoch(date1)
	if dailyUsages[0].EpochDay != expectedEpochDay {
		t.Fatalf("Expected epoch day %d, but got %d", expectedEpochDay, dailyUsages[0].EpochDay)
	}
}

func TestGetUsagePerDay_1Sub_OutOfRangeDates(t *testing.T) {
	database := setupTestDB(t)

	imsi := "001010100007487"

	_, err := createDataNetworkPolicyAndSubscriber(database, imsi)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-1 * 24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -10)
	endDate := time.Now().AddDate(0, 0, -5)

	dailyUsages, err := database.GetUsagePerDay(context.Background(), "", startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage for period: %s", err)
	}

	if len(dailyUsages) != 0 {
		t.Fatalf("Expected 0 daily usage entries, but got %d", len(dailyUsages))
	}
}

func TestGetUsagePerDay_MultiSubsSameDay(t *testing.T) {
	database := setupTestDB(t)

	imsi1 := "001010100007487"

	policyID, err := createDataNetworkPolicyAndSubscriber(database, imsi1)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi1,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	imsi2 := "001010100007488"
	subscriber := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      policyID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete create subscriber 2: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi2,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	dailyUsages, err := database.GetUsagePerDay(context.Background(), "", startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage for period: %s", err)
	}

	if len(dailyUsages) != 1 {
		t.Fatalf("Expected 1 daily usage entry, but got %d", len(dailyUsages))
	}

	if dailyUsages[0].BytesUplink != 2000 {
		t.Fatalf("Expected 2000 uplink bytes, but got %d", dailyUsages[0].BytesUplink)
	}

	if dailyUsages[0].BytesDownlink != 4000 {
		t.Fatalf("Expected 4000 downlink bytes, but got %d", dailyUsages[0].BytesDownlink)
	}

	expectedEpochDay := db.DaysSinceEpoch(date1)
	if dailyUsages[0].EpochDay != expectedEpochDay {
		t.Fatalf("Expected epoch day %d, but got %d", expectedEpochDay, dailyUsages[0].EpochDay)
	}
}

func TestGetUsagePerDay_MultiSubsMultiDays(t *testing.T) {
	database := setupTestDB(t)

	imsi1 := "001010100007487"

	_, err := createDataNetworkPolicyAndSubscriber(database, imsi1)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-48 * time.Hour)
	date2 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi1,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          imsi1,
		BytesUplink:   1500,
		BytesDownlink: 2500,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	dailyUsages, err := database.GetUsagePerDay(context.Background(), "", startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage for period: %s", err)
	}

	if len(dailyUsages) != 2 {
		t.Fatalf("Expected 2 daily usage entries, but got %d", len(dailyUsages))
	}

	expectedEpochDay1 := db.DaysSinceEpoch(date1)
	expectedEpochDay2 := db.DaysSinceEpoch(date2)

	if dailyUsages[0].EpochDay != expectedEpochDay1 {
		t.Fatalf("Expected epoch day %d, but got %d", expectedEpochDay1, dailyUsages[0].EpochDay)
	}

	if dailyUsages[0].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", dailyUsages[0].BytesUplink)
	}

	if dailyUsages[0].BytesDownlink != 2000 {
		t.Fatalf("Expected 2000 downlink bytes, but got %d", dailyUsages[0].BytesDownlink)
	}

	// validate second entry (newer date)
	if dailyUsages[1].EpochDay != expectedEpochDay2 {
		t.Fatalf("Expected epoch day %d, but got %d", expectedEpochDay2, dailyUsages[1].EpochDay)
	}

	if dailyUsages[1].BytesUplink != 1500 {
		t.Fatalf("Expected 1500 uplink bytes, but got %d", dailyUsages[1].BytesUplink)
	}

	if dailyUsages[1].BytesDownlink != 2500 {
		t.Fatalf("Expected 2500 downlink bytes, but got %d", dailyUsages[1].BytesDownlink)
	}
}

func TestGetUsagePerDay_MultiSubsSameDay_FilterByIMSI(t *testing.T) {
	database := setupTestDB(t)

	imsi1 := "001010100007487"

	policyID, err := createDataNetworkPolicyAndSubscriber(database, imsi1)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-48 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi1,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	imsi2 := "001010100007488"
	subscriber := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000022",
		PermanentKey:   "1234567890abcdef1234567890abcdef",
		Opc:            "1234567890abcdef1234567890abcdef",
		ProfileID:      policyID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete create subscriber 2: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi2,
		BytesUplink:   1500,
		BytesDownlink: 2500,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	dailyUsages, err := database.GetUsagePerDay(context.Background(), imsi2, startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage for period: %s", err)
	}

	if len(dailyUsages) != 1 {
		t.Fatalf("Expected 1 daily usage entry, but got %d", len(dailyUsages))
	}

	expectedEpochDay1 := db.DaysSinceEpoch(date1)
	if dailyUsages[0].EpochDay != expectedEpochDay1 {
		t.Fatalf("Expected epoch day %d, but got %d", expectedEpochDay1, dailyUsages[0].EpochDay)
	}

	if dailyUsages[0].BytesUplink != 1500 {
		t.Fatalf("Expected 1500 uplink bytes, but got %d", dailyUsages[0].BytesUplink)
	}

	if dailyUsages[0].BytesDownlink != 2500 {
		t.Fatalf("Expected 2500 downlink bytes, but got %d", dailyUsages[0].BytesDownlink)
	}
}

func TestGetUsagePerDay_MultiSubsMultiDays_FilterByIMSI(t *testing.T) {
	database := setupTestDB(t)

	imsi1 := "001010100007487"

	policyID, err := createDataNetworkPolicyAndSubscriber(database, imsi1)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-48 * time.Hour)
	date2 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi1,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	imsi2 := "001010100007488"
	subscriber := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000022",
		PermanentKey:   "1234567890abcdef1234567890abcdef",
		Opc:            "1234567890abcdef1234567890abcdef",
		ProfileID:      policyID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete create subscriber 2: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi2,
		BytesUplink:   1500,
		BytesDownlink: 2500,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          imsi1,
		BytesUplink:   1222,
		BytesDownlink: 23222,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	dailyUsages, err := database.GetUsagePerDay(context.Background(), imsi1, startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage for period: %s", err)
	}

	if len(dailyUsages) != 2 {
		t.Fatalf("Expected 2 daily usage entries, but got %d", len(dailyUsages))
	}

	expectedEpochDay1 := db.DaysSinceEpoch(date1)
	if dailyUsages[0].EpochDay != expectedEpochDay1 {
		t.Fatalf("Expected epoch day %d, but got %d", expectedEpochDay1, dailyUsages[0].EpochDay)
	}

	if dailyUsages[0].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", dailyUsages[0].BytesUplink)
	}

	if dailyUsages[0].BytesDownlink != 2000 {
		t.Fatalf("Expected 2000 downlink bytes, but got %d", dailyUsages[0].BytesDownlink)
	}

	expectedEpochDay2 := db.DaysSinceEpoch(date2)
	if dailyUsages[1].EpochDay != expectedEpochDay2 {
		t.Fatalf("Expected epoch day %d, but got %d", expectedEpochDay2, dailyUsages[1].EpochDay)
	}

	if dailyUsages[1].BytesUplink != 1222 {
		t.Fatalf("Expected 1222 uplink bytes, but got %d", dailyUsages[1].BytesUplink)
	}

	if dailyUsages[1].BytesDownlink != 23222 {
		t.Fatalf("Expected 23222 downlink bytes, but got %d", dailyUsages[1].BytesDownlink)
	}
}

func TestGetUsagePerSubscriber_1Sub(t *testing.T) {
	database := setupTestDB(t)

	imsi1 := "001010100007488"

	_, err := createDataNetworkPolicyAndSubscriber(database, imsi1)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi1,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	usagePerSubscriber, err := database.GetUsagePerSubscriber(context.Background(), "", startDate, endDate, db.NoUsageLimit)
	if err != nil {
		t.Fatalf("couldn't get daily usage per subscriber for period: %s", err)
	}

	if len(usagePerSubscriber) != 1 {
		t.Fatalf("Expected 1 usage per subscriber entry, but got %d", len(usagePerSubscriber))
	}

	if usagePerSubscriber[0].IMSI != imsi1 {
		t.Fatalf("Expected IMSI 'test_imsi_1', but got %s", usagePerSubscriber[0].IMSI)
	}

	if usagePerSubscriber[0].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", usagePerSubscriber[0].BytesUplink)
	}

	if usagePerSubscriber[0].BytesDownlink != 2000 {
		t.Fatalf("Expected 2000 downlink bytes, but got %d", usagePerSubscriber[0].BytesDownlink)
	}
}

func TestGetUsagePerSubscriber_MultiSub(t *testing.T) {
	database := setupTestDB(t)

	imsi1 := "001010100007487"

	policyID, err := createDataNetworkPolicyAndSubscriber(database, imsi1)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	date1 := time.Now().Add(-24 * time.Hour)
	date2 := time.Now().Add(-48 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi1,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	imsi2 := "001010100007488"
	subscriber := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000022",
		PermanentKey:   "1234567890abcdef1234567890abcdef",
		Opc:            "1234567890abcdef1234567890abcdef",
		ProfileID:      policyID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete create subscriber 2: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi2,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	imsi3 := "001010100007489"
	subscriber = &db.Subscriber{
		Imsi:           imsi3,
		SequenceNumber: "000000000022",
		PermanentKey:   "1234567890abcdef1234567890abcdef",
		Opc:            "1234567890abcdef1234567890abcdef",
		ProfileID:      policyID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete create subscriber 3: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          imsi3,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          imsi3,
		BytesUplink:   3333,
		BytesDownlink: 4444,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	usagePerSubscriber, err := database.GetUsagePerSubscriber(context.Background(), "", startDate, endDate, db.NoUsageLimit)
	if err != nil {
		t.Fatalf("couldn't get daily usage per subscriber for period: %s", err)
	}

	if len(usagePerSubscriber) != 3 {
		t.Fatalf("Expected 3 usage per subscriber entries, but got %d", len(usagePerSubscriber))
	}

	if usagePerSubscriber[0].IMSI != imsi3 {
		t.Fatalf("Expected IMSI '%s', but got %s", imsi3, usagePerSubscriber[0].IMSI)
	}

	if usagePerSubscriber[0].BytesUplink != 4333 {
		t.Fatalf("Expected 4333 uplink bytes, but got %d", usagePerSubscriber[0].BytesUplink)
	}

	if usagePerSubscriber[0].BytesDownlink != 6444 {
		t.Fatalf("Expected 6444 downlink bytes, but got %d", usagePerSubscriber[0].BytesDownlink)
	}

	if usagePerSubscriber[1].IMSI != imsi2 {
		t.Fatalf("Expected IMSI '%s', but got %s", imsi2, usagePerSubscriber[1].IMSI)
	}

	if usagePerSubscriber[1].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", usagePerSubscriber[1].BytesUplink)
	}

	if usagePerSubscriber[1].BytesDownlink != 2000 {
		t.Fatalf("Expected 2000 downlink bytes, but got %d", usagePerSubscriber[1].BytesDownlink)
	}

	if usagePerSubscriber[2].IMSI != imsi1 {
		t.Fatalf("Expected IMSI '%s', but got %s", imsi1, usagePerSubscriber[2].IMSI)
	}

	if usagePerSubscriber[2].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", usagePerSubscriber[2].BytesUplink)
	}
}

func TestClearDailyUsage(t *testing.T) {
	database := setupTestDB(t)

	date := time.Now()

	testImsi := "001010100007487"

	_, err := createDataNetworkPolicyAndSubscriber(database, testImsi)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date),
		IMSI:          testImsi,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	dailyUsage, err := database.GetUsagePerDay(context.Background(), testImsi, date, date)
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if len(dailyUsage) == 0 {
		t.Fatalf("Expected a daily usage entry, but got none")
	}

	err = database.ClearDailyUsage(context.Background())
	if err != nil {
		t.Fatalf("couldn't clear daily usage: %s", err)
	}

	dailyUsage, err = database.GetUsagePerDay(context.Background(), testImsi, date, date)
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if len(dailyUsage) != 0 {
		t.Fatalf("Expected no daily usage entry, but got one: %+v", dailyUsage)
	}
}

func TestDeleteOldDailyUsage(t *testing.T) {
	database := setupTestDB(t)

	testImsi := "001010100007487"

	_, err := createDataNetworkPolicyAndSubscriber(database, testImsi)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	oldDate := time.Now().AddDate(0, 0, -10)
	newDate := time.Now()

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(oldDate),
		IMSI:          testImsi,
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(newDate),
		IMSI:          testImsi,
		BytesUplink:   3000,
		BytesDownlink: 4000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.DeleteOldDailyUsage(context.Background(), 5)
	if err != nil {
		t.Fatalf("couldn't delete old daily usage: %s", err)
	}

	dailyUsage, err := database.GetUsagePerDay(context.Background(), testImsi, oldDate, oldDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if len(dailyUsage) != 0 {
		t.Fatalf("Expected no old daily usage entry, but got one: %+v", dailyUsage)
	}

	dailyUsage, err = database.GetUsagePerDay(context.Background(), testImsi, newDate, newDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if len(dailyUsage) == 0 {
		t.Fatalf("Expected a new daily usage entry, but got none")
	}
}

func TestGetUsagePerSubscriber_Limit(t *testing.T) {
	database := setupTestDB(t)

	imsis := []string{"001010100007501", "001010100007502", "001010100007503"}

	profileID, err := createDataNetworkPolicyAndSubscriber(database, imsis[0])
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetworkPolicyAndSubscriber: %s", err)
	}

	for _, imsi := range imsis[1:] {
		err = database.CreateSubscriber(context.Background(), &db.Subscriber{
			Imsi:           imsi,
			SequenceNumber: "000000000022",
			PermanentKey:   "1234567890abcdef1234567890abcdef",
			Opc:            "1234567890abcdef1234567890abcdef",
			ProfileID:      profileID,
		})
		if err != nil {
			t.Fatalf("Couldn't create subscriber %s: %s", imsi, err)
		}
	}

	date := time.Now().Add(-24 * time.Hour)

	// Ascending totals, so the limited result must come back reversed.
	for i, imsi := range imsis {
		err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
			EpochDay:      db.DaysSinceEpoch(date),
			IMSI:          imsi,
			BytesUplink:   int64(100 * (i + 1)),
			BytesDownlink: int64(200 * (i + 1)),
		})
		if err != nil {
			t.Fatalf("couldn't increment daily usage: %s", err)
		}
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	all, err := database.GetUsagePerSubscriber(context.Background(), "", startDate, endDate, db.NoUsageLimit)
	if err != nil {
		t.Fatalf("Couldn't complete GetUsagePerSubscriber: %s", err)
	}

	if len(all) != 3 {
		t.Fatalf("expected 3 subscribers with NoUsageLimit, got %d", len(all))
	}

	if all[0].IMSI != imsis[2] {
		t.Fatalf("expected highest-usage subscriber %s first, got %s", imsis[2], all[0].IMSI)
	}

	limited, err := database.GetUsagePerSubscriber(context.Background(), "", startDate, endDate, 2)
	if err != nil {
		t.Fatalf("Couldn't complete GetUsagePerSubscriber with limit: %s", err)
	}

	if len(limited) != 2 {
		t.Fatalf("expected 2 subscribers with limit 2, got %d", len(limited))
	}

	// The limit must keep the top of the existing total-bytes ordering, not an
	// arbitrary two rows.
	if limited[0].IMSI != imsis[2] || limited[1].IMSI != imsis[1] {
		t.Fatalf("expected top-2 by total bytes (%s, %s), got (%s, %s)", imsis[2], imsis[1], limited[0].IMSI, limited[1].IMSI)
	}
}

func TestIncrementDailyUsageBatch_AccumulatesEveryRow(t *testing.T) {
	database := setupTestDB(t)

	profileID, err := createDataNetworkPolicyAndSubscriber(database, "001010000000001")
	if err != nil {
		t.Fatalf("Couldn't create fixtures: %s", err)
	}

	imsis := []string{"001010000000001", "001010000000002", "001010000000003"}
	for _, imsi := range imsis[1:] {
		if err := database.CreateSubscriber(context.Background(), &db.Subscriber{
			Imsi:           imsi,
			SequenceNumber: "000000000022",
			PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
			Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
			ProfileID:      profileID,
		}); err != nil {
			t.Fatalf("Couldn't create subscriber: %s", err)
		}
	}

	day := db.DaysSinceEpoch(time.Now())

	rows := make([]db.DailyUsage, len(imsis))
	for i, imsi := range imsis {
		rows[i] = db.DailyUsage{EpochDay: day, IMSI: imsi, BytesUplink: 100, BytesDownlink: 200}
	}

	for range 2 {
		if err := database.IncrementDailyUsageBatch(context.Background(), rows); err != nil {
			t.Fatalf("Couldn't complete IncrementDailyUsageBatch: %s", err)
		}
	}

	for _, imsi := range imsis {
		usage, err := database.GetUsagePerSubscriber(context.Background(), imsi, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour), db.NoUsageLimit)
		if err != nil {
			t.Fatalf("Couldn't complete GetUsagePerSubscriber: %s", err)
		}

		if len(usage) != 1 {
			t.Fatalf("got %d usage rows for %s, want 1", len(usage), imsi)
		}

		if usage[0].BytesUplink != 200 || usage[0].BytesDownlink != 400 {
			t.Errorf("got up=%d down=%d for %s, want 200/400", usage[0].BytesUplink, usage[0].BytesDownlink, imsi)
		}
	}
}

func TestIncrementDailyUsageBatch_SkipsUnknownSubscriberAndKeepsTheRest(t *testing.T) {
	database := setupTestDB(t)

	if _, err := createDataNetworkPolicyAndSubscriber(database, "001010000000001"); err != nil {
		t.Fatalf("Couldn't create fixtures: %s", err)
	}

	day := db.DaysSinceEpoch(time.Now())

	rows := []db.DailyUsage{
		{EpochDay: day, IMSI: "001019999999999", BytesUplink: 100, BytesDownlink: 200},
		{EpochDay: day, IMSI: "001010000000001", BytesUplink: 100, BytesDownlink: 200},
	}

	if err := database.IncrementDailyUsageBatch(context.Background(), rows); err != nil {
		t.Fatalf("a batch with an unknown subscriber must still record the others: %s", err)
	}

	usage, err := database.GetUsagePerSubscriber(context.Background(), "001010000000001", time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour), db.NoUsageLimit)
	if err != nil {
		t.Fatalf("Couldn't complete GetUsagePerSubscriber: %s", err)
	}

	if len(usage) != 1 || usage[0].BytesUplink != 100 || usage[0].BytesDownlink != 200 {
		t.Fatalf("the known subscriber's usage was lost: %+v", usage)
	}

	orphan, err := database.GetUsagePerSubscriber(context.Background(), "001019999999999", time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour), db.NoUsageLimit)
	if err != nil {
		t.Fatalf("Couldn't complete GetUsagePerSubscriber: %s", err)
	}

	if len(orphan) != 0 {
		t.Errorf("got %d rows for the unknown subscriber, want none", len(orphan))
	}
}

func TestIncrementDailyUsageBatch_EmptyIsANoop(t *testing.T) {
	database := setupTestDB(t)

	if err := database.IncrementDailyUsageBatch(context.Background(), nil); err != nil {
		t.Fatalf("an empty batch must be a no-op: %s", err)
	}
}

func setupRaftTestDB(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabase(context.Background(),
		filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	})

	return database
}

func TestIncrementDailyUsageBatch_AFailedRowRollsBackTheRowsBeforeIt(t *testing.T) {
	database := setupRaftTestDB(t)

	profileID, err := createDataNetworkPolicyAndSubscriber(database, "001010000000001")
	if err != nil {
		t.Fatalf("Couldn't create fixtures: %s", err)
	}

	if err := database.CreateSubscriber(context.Background(), &db.Subscriber{
		Imsi:           "001010000000002",
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
	}); err != nil {
		t.Fatalf("Couldn't create subscriber: %s", err)
	}

	day := db.DaysSinceEpoch(time.Now())

	rows := []db.DailyUsage{
		{EpochDay: day, IMSI: "001010000000001", BytesUplink: 111, BytesDownlink: 222},
		{EpochDay: day, IMSI: "001010000000002", BytesUplink: -1, BytesDownlink: 0},
	}

	if err := database.IncrementDailyUsageBatch(context.Background(), rows); err == nil {
		t.Fatal("a CHECK violation must fail the batch")
	}

	usage, err := database.GetUsagePerSubscriber(context.Background(), "", time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour), db.NoUsageLimit)
	if err != nil {
		t.Fatalf("Couldn't complete GetUsagePerSubscriber: %s", err)
	}

	for _, row := range usage {
		if row.BytesUplink != 0 || row.BytesDownlink != 0 {
			t.Fatalf("the row before the failing one was persisted: %+v", usage)
		}
	}
}

func skippedDailyUsageRows(t *testing.T) float64 {
	t.Helper()

	ch := make(chan prometheus.Metric, 1)
	db.DailyUsageRowsSkipped.Collect(ch)
	close(ch)

	var m dto.Metric

	if err := (<-ch).Write(&m); err != nil {
		t.Fatalf("read DailyUsageRowsSkipped: %s", err)
	}

	return m.GetCounter().GetValue()
}

func TestIncrementDailyUsageBatch_CountsSkippedRowsOnce(t *testing.T) {
	database := setupRaftTestDB(t)

	if _, err := createDataNetworkPolicyAndSubscriber(database, "001010000000001"); err != nil {
		t.Fatalf("Couldn't create fixtures: %s", err)
	}

	day := db.DaysSinceEpoch(time.Now())

	rows := []db.DailyUsage{
		{EpochDay: day, IMSI: "001019999999998", BytesUplink: 100, BytesDownlink: 200},
		{EpochDay: day, IMSI: "001019999999999", BytesUplink: 100, BytesDownlink: 200},
		{EpochDay: day, IMSI: "001010000000001", BytesUplink: 100, BytesDownlink: 200},
	}

	before := skippedDailyUsageRows(t)

	if err := database.IncrementDailyUsageBatch(context.Background(), rows); err != nil {
		t.Fatalf("Couldn't complete IncrementDailyUsageBatch: %s", err)
	}

	if got := skippedDailyUsageRows(t) - before; got != 2 {
		t.Fatalf("counted %v skipped rows, want 2", got)
	}
}

func TestIncrementDailyUsageBatch_CountsNoSkippedRowWhenTheBatchFails(t *testing.T) {
	database := setupRaftTestDB(t)

	if _, err := createDataNetworkPolicyAndSubscriber(database, "001010000000001"); err != nil {
		t.Fatalf("Couldn't create fixtures: %s", err)
	}

	day := db.DaysSinceEpoch(time.Now())

	rows := []db.DailyUsage{
		{EpochDay: day, IMSI: "001019999999999", BytesUplink: 100, BytesDownlink: 200},
		{EpochDay: day, IMSI: "001010000000001", BytesUplink: -1, BytesDownlink: 0},
	}

	before := skippedDailyUsageRows(t)

	if err := database.IncrementDailyUsageBatch(context.Background(), rows); err == nil {
		t.Fatal("a CHECK violation must fail the batch")
	}

	if got := skippedDailyUsageRows(t) - before; got != 0 {
		t.Fatalf("a batch that never committed counted %v skipped rows, want 0", got)
	}
}

func TestIncrementDailyUsageBatch_AbortsOnAnErrorThatIsNotAMissingSubscriber(t *testing.T) {
	database := setupTestDB(t)

	if _, err := createDataNetworkPolicyAndSubscriber(database, "001010000000001"); err != nil {
		t.Fatalf("Couldn't create fixtures: %s", err)
	}

	day := db.DaysSinceEpoch(time.Now())

	rows := []db.DailyUsage{
		{EpochDay: day, IMSI: "001010000000001", BytesUplink: -1, BytesDownlink: 200},
	}

	if err := database.IncrementDailyUsageBatch(context.Background(), rows); err == nil {
		t.Fatal("a CHECK violation must fail the batch, not be skipped like a missing subscriber")
	}

	usage, err := database.GetUsagePerSubscriber(context.Background(), "001010000000001", time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour), db.NoUsageLimit)
	if err != nil {
		t.Fatalf("Couldn't complete GetUsagePerSubscriber: %s", err)
	}

	if len(usage) != 0 {
		t.Errorf("got %d rows after a failed batch, want none", len(usage))
	}
}
