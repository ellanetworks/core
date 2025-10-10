// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetAndIncrementDailyUsageEndToEnd(t *testing.T) {
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

	dailyUsage, err := database.GetDailyUsage(context.Background(), date, "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if dailyUsage != nil {
		t.Fatalf("Expected no daily usage entry, but got one: %+v", dailyUsage)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date),
		IMSI:          "test_imsi",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	dailyUsage, err = database.GetDailyUsage(context.Background(), date, "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if dailyUsage == nil {
		t.Fatalf("Expected a daily usage entry, but got none")
	}

	if dailyUsage.BytesUplink != 1000 {
		t.Fatalf("Expected 1000 uplink bytes, but got %d", dailyUsage.BytesUplink)
	}

	if dailyUsage.BytesDownlink != 2000 {
		t.Fatalf("Expected 2000 downlink bytes, but got %d", dailyUsage.BytesDownlink)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date),
		IMSI:          "test_imsi",
		BytesUplink:   500,
		BytesDownlink: 1500,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	dailyUsage, err = database.GetDailyUsage(context.Background(), date, "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if dailyUsage == nil {
		t.Fatalf("Expected a daily usage entry, but got none")
	}

	if dailyUsage.BytesUplink != 1500 {
		t.Fatalf("Expected 1500 uplink bytes, but got %d", dailyUsage.BytesUplink)
	}

	if dailyUsage.BytesDownlink != 3500 {
		t.Fatalf("Expected 3500 downlink bytes, but got %d", dailyUsage.BytesDownlink)
	}
}

func TestGetTotalUsageForIMSI(t *testing.T) {
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

	date1 := time.Now()
	date2 := date1.Add(24 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          "test_imsi",
		BytesUplink:   3000,
		BytesDownlink: 4000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	totalUsage, err := database.GetTotalUsageForIMSI(context.Background(), "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get total usage for IMSI: %s", err)
	}

	if totalUsage == nil {
		t.Fatalf("Expected total usage entry, but got none")
	}

	if totalUsage.BytesUplink != 4000 {
		t.Fatalf("Expected 4000 uplink bytes, but got %d", totalUsage.BytesUplink)
	}

	if totalUsage.BytesDownlink != 6000 {
		t.Fatalf("Expected 6000 downlink bytes, but got %d", totalUsage.BytesDownlink)
	}
}

func TestGetUsageForPeriod(t *testing.T) {
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

	date1 := time.Now()
	date2 := date1.Add(24 * time.Hour)
	date3 := date1.Add(48 * time.Hour)

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date1),
		IMSI:          "test_imsi",
		BytesUplink:   1000,
		BytesDownlink: 2000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date2),
		IMSI:          "test_imsi",
		BytesUplink:   3000,
		BytesDownlink: 4000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	err = database.IncrementDailyUsage(context.Background(), db.DailyUsage{
		EpochDay:      db.DaysSinceEpoch(date3),
		IMSI:          "test_imsi",
		BytesUplink:   5000,
		BytesDownlink: 6000,
	})
	if err != nil {
		t.Fatalf("couldn't increment daily usage: %s", err)
	}

	totalUsage, err := database.GetUsageForPeriod(context.Background(), "test_imsi", date1, date3)
	if err != nil {
		t.Fatalf("couldn't get usage for period: %s", err)
	}

	if totalUsage == nil {
		t.Fatalf("Expected total usage entry, but got none")
	}

	if totalUsage.BytesUplink != 9000 {
		t.Fatalf("Expected 9000 uplink bytes, but got %d", totalUsage.BytesUplink)
	}

	if totalUsage.BytesDownlink != 12000 {
		t.Fatalf("Expected 12000 downlink bytes, but got %d", totalUsage.BytesDownlink)
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

	dailyUsage, err := database.GetDailyUsage(context.Background(), date, "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if dailyUsage == nil {
		t.Fatalf("Expected a daily usage entry, but got none")
	}

	err = database.ClearDailyUsage(context.Background())
	if err != nil {
		t.Fatalf("couldn't clear daily usage: %s", err)
	}

	dailyUsage, err = database.GetDailyUsage(context.Background(), date, "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if dailyUsage != nil {
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

	dailyUsage, err := database.GetDailyUsage(context.Background(), oldDate, "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if dailyUsage != nil {
		t.Fatalf("Expected no old daily usage entry, but got one: %+v", dailyUsage)
	}

	dailyUsage, err = database.GetDailyUsage(context.Background(), newDate, "test_imsi")
	if err != nil {
		t.Fatalf("couldn't get daily usage: %s", err)
	}

	if dailyUsage == nil {
		t.Fatalf("Expected a new daily usage entry, but got none")
	}
}
