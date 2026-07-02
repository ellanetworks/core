// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestPositioningSessionCreateDelete(t *testing.T) {
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

	session := &db.PositioningSession{
		SUPI:        "imsi-001010000000001",
		SessionType: db.SessionTypeImmediate,
		Method:      "cell_id",
		Status:      db.SessionStatusActive,
	}

	if err := database.CreatePositioningSession(context.Background(), session); err != nil {
		t.Fatalf("Couldn't complete CreatePositioningSession: %s", err)
	}

	if session.ID == "" {
		t.Fatalf("Expected non-empty ID from CreatePositioningSession")
	}

	// The session must exist before deletion.
	if _, err := database.GetPositioningSession(context.Background(), session.ID); err != nil {
		t.Fatalf("Couldn't complete GetPositioningSession: %s", err)
	}

	if err := database.DeletePositioningSession(context.Background(), session.ID); err != nil {
		t.Fatalf("Couldn't complete DeletePositioningSession: %s", err)
	}

	// After deletion the session must no longer be retrievable.
	if _, err := database.GetPositioningSession(context.Background(), session.ID); err == nil {
		t.Fatalf("Expected error retrieving deleted positioning session, got nil")
	}
}

func TestPositioningSessionDeleteNotFound(t *testing.T) {
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

	err = database.DeletePositioningSession(context.Background(), "non-existent-id")
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound deleting non-existent session, got: %v", err)
	}
}
