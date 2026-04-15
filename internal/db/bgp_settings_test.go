package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestGetBGPSettings_Default(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	settings, err := database.GetBGPSettings(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get BGP settings: %s", err)
	}

	if settings.Enabled {
		t.Fatalf("BGP should be disabled by default")
	}

	if settings.LocalAS != 64512 {
		t.Fatalf("Expected default localAS 64512, got %d", settings.LocalAS)
	}

	if settings.RouterID != "" {
		t.Fatalf("Expected empty default routerID, got %s", settings.RouterID)
	}
}

func TestIsBGPEnabled_Default(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	enabled, err := database.IsBGPEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't check IsBGPEnabled: %s", err)
	}

	if enabled {
		t.Fatalf("BGP should be disabled by default")
	}
}

func TestUpdateAndGetBGPSettings(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	newSettings := &db.BGPSettings{
		Enabled:  true,
		LocalAS:  64513,
		RouterID: "192.168.1.1",
	}

	err = database.UpdateBGPSettings(context.Background(), newSettings)
	if err != nil {
		t.Fatalf("Couldn't update BGP settings: %s", err)
	}

	settings, err := database.GetBGPSettings(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get BGP settings: %s", err)
	}

	if !settings.Enabled {
		t.Fatalf("BGP should be enabled")
	}

	if settings.LocalAS != 64513 {
		t.Fatalf("Expected localAS 64513, got %d", settings.LocalAS)
	}

	if settings.RouterID != "192.168.1.1" {
		t.Fatalf("Expected routerID 192.168.1.1, got %s", settings.RouterID)
	}

	// Update again to disable
	disabledSettings := &db.BGPSettings{
		Enabled:  false,
		LocalAS:  64512,
		RouterID: "",
	}

	err = database.UpdateBGPSettings(context.Background(), disabledSettings)
	if err != nil {
		t.Fatalf("Couldn't update BGP settings: %s", err)
	}

	settings, err = database.GetBGPSettings(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get BGP settings: %s", err)
	}

	if settings.Enabled {
		t.Fatalf("BGP should be disabled")
	}

	if settings.LocalAS != 64512 {
		t.Fatalf("Expected localAS 64512, got %d", settings.LocalAS)
	}
}

func TestUpdateBGPSettings_RestartDatabase(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	newSettings := &db.BGPSettings{
		Enabled:  true,
		LocalAS:  65000,
		RouterID: "10.0.0.1",
	}

	err = database.UpdateBGPSettings(context.Background(), newSettings)
	if err != nil {
		t.Fatalf("Couldn't update BGP settings: %s", err)
	}

	err = database.Close()
	if err != nil {
		t.Fatalf("Couldn't complete Close: %s", err)
	}

	database, err = db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	settings, err := database.GetBGPSettings(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get BGP settings: %s", err)
	}

	if !settings.Enabled {
		t.Fatalf("BGP should be enabled after restart")
	}

	if settings.LocalAS != 65000 {
		t.Fatalf("Expected localAS 65000, got %d", settings.LocalAS)
	}

	if settings.RouterID != "10.0.0.1" {
		t.Fatalf("Expected routerID 10.0.0.1, got %s", settings.RouterID)
	}
}
