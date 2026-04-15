// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestJWTSecretInitialize(t *testing.T) {
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

	// NewDatabase calls Initialize which calls InitializeJWTSecret,
	// so a secret should already exist.
	secret, err := database.GetJWTSecret(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get JWT secret: %s", err)
	}

	if len(secret) != 32 {
		t.Fatalf("Expected JWT secret length 32, got %d", len(secret))
	}

	// Calling InitializeJWTSecret again should be a no-op (same secret).
	err = database.InitializeJWTSecret(context.Background())
	if err != nil {
		t.Fatalf("InitializeJWTSecret failed on second call: %s", err)
	}

	secret2, err := database.GetJWTSecret(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get JWT secret after re-initialize: %s", err)
	}

	if string(secret) != string(secret2) {
		t.Fatalf("JWT secret changed after re-initialize")
	}
}

func TestJWTSecretSetAndGet(t *testing.T) {
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

	newSecret := []byte("my-custom-secret-key-for-testing!")

	err = database.SetJWTSecret(context.Background(), newSecret)
	if err != nil {
		t.Fatalf("Couldn't set JWT secret: %s", err)
	}

	retrieved, err := database.GetJWTSecret(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get JWT secret: %s", err)
	}

	if string(retrieved) != string(newSecret) {
		t.Fatalf("Expected JWT secret %q, got %q", newSecret, retrieved)
	}
}

func TestJWTSecretPersistsAcrossReopen(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	secret, err := database.GetJWTSecret(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get JWT secret: %s", err)
	}

	if err := database.Close(); err != nil {
		t.Fatalf("Couldn't complete Close: %s", err)
	}

	// Re-open the same database file.
	database2, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase on reopen: %s", err)
	}

	defer func() {
		if err := database2.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	secret2, err := database2.GetJWTSecret(context.Background())
	if err != nil {
		t.Fatalf("Couldn't get JWT secret after reopen: %s", err)
	}

	if string(secret) != string(secret2) {
		t.Fatalf("JWT secret changed after database reopen")
	}
}
