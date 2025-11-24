// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetUsagePerDay_1Sub(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi",
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
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-1 * 24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi",
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
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_1",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_2",
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
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-48 * time.Hour)
	date2 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_1",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          "test_imsi_1",
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
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-48 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_1",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_2",
		BytesUplink:   1500,
		BytesDownlink: 2500,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	dailyUsages, err := database.GetUsagePerDay(context.Background(), "test_imsi_2", startDate, endDate)
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
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-48 * time.Hour)
	date2 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_1",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_2",
		BytesUplink:   1500,
		BytesDownlink: 2500,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          "test_imsi_1",
		BytesUplink:   1222,
		BytesDownlink: 23222,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	dailyUsages, err := database.GetUsagePerDay(context.Background(), "test_imsi_1", startDate, endDate)
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
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_1",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	usagePerSubscriber, err := database.GetUsagePerSubscriber(context.Background(), "", startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage per subscriber for period: %s", err)
	}

	if len(usagePerSubscriber) != 1 {
		t.Fatalf("Expected 1 usage per subscriber entry, but got %d", len(usagePerSubscriber))
	}

	if usagePerSubscriber[0].IMSI != "test_imsi_1" {
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
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date1 := time.Now().Add(-24 * time.Hour)
	date2 := time.Now().Add(-48 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_1",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_2",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi_3",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          "test_imsi_3",
		BytesUplink:   3333,
		BytesDownlink: 4444,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	startDate := time.Now().AddDate(0, 0, -5)
	endDate := time.Now()

	usagePerSubscriber, err := database.GetUsagePerSubscriber(context.Background(), "", startDate, endDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage per subscriber for period: %s", err)
	}

	if len(usagePerSubscriber) != 3 {
		t.Fatalf("Expected 3 usage per subscriber entries, but got %d", len(usagePerSubscriber))
	}

	if usagePerSubscriber[0].IMSI != "test_imsi_3" {
		t.Fatalf("Expected IMSI 'test_imsi_3', but got %s", usagePerSubscriber[0].IMSI)
	}

	if usagePerSubscriber[0].BytesUplink != 4333 {
		t.Fatalf("Expected 4333 uplink bytes, but got %d", usagePerSubscriber[0].BytesUplink)
	}

	if usagePerSubscriber[0].BytesDownlink != 6444 {
		t.Fatalf("Expected 6444 downlink bytes, but got %d", usagePerSubscriber[0].BytesDownlink)
	}

	if usagePerSubscriber[1].IMSI != "test_imsi_2" {
		t.Fatalf("Expected IMSI 'test_imsi_2', but got %s", usagePerSubscriber[1].IMSI)
	}

	if usagePerSubscriber[1].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", usagePerSubscriber[1].BytesUplink)
	}

	if usagePerSubscriber[1].BytesDownlink != 2000 {
		t.Fatalf("Expected 2000 downlink bytes, but got %d", usagePerSubscriber[1].BytesDownlink)
	}

	if usagePerSubscriber[2].IMSI != "test_imsi_1" {
		t.Fatalf("Expected IMSI 'test_imsi_1', but got %s", usagePerSubscriber[2].IMSI)
	}

	if usagePerSubscriber[2].BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", usagePerSubscriber[2].BytesUplink)
	}
}

func TestClearDailyUsage(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	date := time.Now()

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date),
		IMSI:          "test_imsi",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	dailyUsage, err := database.GetUsagePerDay(context.Background(), "test_imsi", date, date)
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

	dailyUsage, err = database.GetUsagePerDay(context.Background(), "test_imsi", date, date)
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if len(dailyUsage) != 0 {
		t.Fatalf("Expected no daily usage entry, but got one: %+v", dailyUsage)
	}
}

func TestDeleteOldDailyUsage(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	oldDate := time.Now().AddDate(0, 0, -10)
	newDate := time.Now()

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(oldDate),
		IMSI:          "test_imsi",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(newDate),
		IMSI:          "test_imsi",
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

	dailyUsage, err := database.GetUsagePerDay(context.Background(), "test_imsi", oldDate, oldDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if len(dailyUsage) != 0 {
		t.Fatalf("Expected no old daily usage entry, but got one: %+v", dailyUsage)
	}

	dailyUsage, err = database.GetUsagePerDay(context.Background(), "test_imsi", newDate, newDate)
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if len(dailyUsage) == 0 {
		t.Fatalf("Expected a new daily usage entry, but got none")
	}
}
