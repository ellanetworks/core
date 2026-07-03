// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func f64(v float64) *float64 { return &v }

func TestCellPositionCRUD(t *testing.T) {
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

	ctx := context.Background()

	cp := &db.CellPosition{
		RAT:                  db.RATNR,
		Mcc:                  "001",
		Mnc:                  "01",
		CellIdentity:         "00066c000",
		Latitude:             45.0,
		Longitude:            21.4577,
		Altitude:             f64(100),
		UncertaintySemiMajor: f64(50),
		UncertaintySemiMinor: f64(50),
	}

	if err := database.CreateCellPosition(ctx, cp); err != nil {
		t.Fatalf("CreateCellPosition: %s", err)
	}

	if cp.ID == "" {
		t.Fatal("expected non-empty ID after create")
	}

	// Lookup by natural (serving-cell) key — the LMF path.
	got, err := database.GetCellPositionByCell(ctx, db.RATNR, "001", "01", "00066c000")
	if err != nil {
		t.Fatalf("GetCellPositionByCell: %s", err)
	}

	if got.Latitude != 45.0 || got.Longitude != 21.4577 {
		t.Errorf("coordinates mismatch: got %f/%f", got.Latitude, got.Longitude)
	}

	// Missing cell returns ErrNotFound.
	if _, err := database.GetCellPositionByCell(ctx, db.RATNR, "001", "01", "deadbeef"); err != db.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing cell, got %v", err)
	}

	// Update.
	got.Latitude = 46.0
	if err := database.UpdateCellPosition(ctx, got); err != nil {
		t.Fatalf("UpdateCellPosition: %s", err)
	}

	after, err := database.GetCellPosition(ctx, cp.ID)
	if err != nil {
		t.Fatalf("GetCellPosition: %s", err)
	}

	if after.Latitude != 46.0 {
		t.Errorf("update not persisted: got lat %f", after.Latitude)
	}

	// List.
	list, err := database.ListCellPositions(ctx)
	if err != nil {
		t.Fatalf("ListCellPositions: %s", err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 cell position, got %d", len(list))
	}

	// Delete.
	if err := database.DeleteCellPosition(ctx, cp.ID); err != nil {
		t.Fatalf("DeleteCellPosition: %s", err)
	}

	if _, err := database.GetCellPositionByCell(ctx, db.RATNR, "001", "01", "00066c000"); err != db.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
