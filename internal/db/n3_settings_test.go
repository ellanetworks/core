// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestN3Settings_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	n3Settings, err := database.GetN3Settings(ctx)
	if err != nil {
		t.Fatalf("Couldn't complete GetN3Settings: %s", err)
	}

	if n3Settings.ExternalAddress != "" {
		t.Fatalf("N3 external address should be empty by default")
	}

	newExternalAddress := "1.2.3.4"
	if err := database.UpdateN3Settings(ctx, newExternalAddress); err != nil {
		t.Fatalf("Couldn't Update N3 Settings: %s", err)
	}

	updatedN3Settings, err := database.GetN3Settings(ctx)
	if err != nil {
		t.Fatalf("Couldn't complete GetN3Settings: %s", err)
	}

	if updatedN3Settings.ExternalAddress != newExternalAddress {
		t.Fatalf("N3 external address was not updated correctly")
	}
}
