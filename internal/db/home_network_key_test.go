// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestHomeNetworkKeysCRUD(t *testing.T) {
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

	// A default Profile A key is seeded during initialization.
	count, err := database.CountHomeNetworkKeys(ctx)
	if err != nil {
		t.Fatalf("CountHomeNetworkKeys failed: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 default key, got %d", count)
	}

	// List should return the default key.
	keys, err := database.ListHomeNetworkKeys(ctx)
	if err != nil {
		t.Fatalf("ListHomeNetworkKeys failed: %s", err)
	}

	if len(keys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(keys))
	}

	if keys[0].Scheme != "A" || keys[0].KeyIdentifier != 0 {
		t.Fatalf("Expected default key (A, 0), got (%s, %d)", keys[0].Scheme, keys[0].KeyIdentifier)
	}

	// Get by ID.
	key, err := database.GetHomeNetworkKey(ctx, keys[0].ID)
	if err != nil {
		t.Fatalf("GetHomeNetworkKey failed: %s", err)
	}

	if key.Scheme != "A" {
		t.Fatalf("Expected scheme A, got %s", key.Scheme)
	}

	// Get by scheme and identifier.
	key, err = database.GetHomeNetworkKeyBySchemeAndIdentifier(ctx, "A", 0)
	if err != nil {
		t.Fatalf("GetHomeNetworkKeyBySchemeAndIdentifier failed: %s", err)
	}

	if key.PrivateKey == "" {
		t.Fatal("Expected non-empty private key")
	}

	// Get public key for Profile A.
	pubKey, err := key.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey failed: %s", err)
	}

	if len(pubKey) != 64 { // 32 bytes = 64 hex chars
		t.Fatalf("Expected 64-char public key, got %d chars", len(pubKey))
	}

	// Create a Profile B key.
	// Use a known valid P-256 private key (from TS 33.501 Annex C.3.4).
	profileBKey := &db.HomeNetworkKey{
		KeyIdentifier: 0,
		Scheme:        "B",
		PrivateKey:    "f1ab1074477ebcce59b97460c83b4071db578ffab54ee4fbc76aeca38e4b7b01",
	}

	err = database.CreateHomeNetworkKey(ctx, profileBKey)
	if err != nil {
		t.Fatalf("CreateHomeNetworkKey (Profile B) failed: %s", err)
	}

	count, err = database.CountHomeNetworkKeys(ctx)
	if err != nil {
		t.Fatalf("CountHomeNetworkKeys failed: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected 2 keys, got %d", count)
	}

	// Get public key for Profile B.
	bKey, err := database.GetHomeNetworkKeyBySchemeAndIdentifier(ctx, "B", 0)
	if err != nil {
		t.Fatalf("GetHomeNetworkKeyBySchemeAndIdentifier (B) failed: %s", err)
	}

	pubKeyB, err := bKey.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey (B) failed: %s", err)
	}

	if len(pubKeyB) != 66 { // 33 bytes = 66 hex chars (compressed)
		t.Fatalf("Expected 66-char compressed public key, got %d chars", len(pubKeyB))
	}

	// Create a second Profile A key (key rotation scenario).
	rotationKey := &db.HomeNetworkKey{
		KeyIdentifier: 1,
		Scheme:        "A",
		PrivateKey:    keys[0].PrivateKey, // reuse for simplicity
	}

	err = database.CreateHomeNetworkKey(ctx, rotationKey)
	if err != nil {
		t.Fatalf("CreateHomeNetworkKey (rotation) failed: %s", err)
	}

	count, err = database.CountHomeNetworkKeys(ctx)
	if err != nil {
		t.Fatalf("CountHomeNetworkKeys failed: %s", err)
	}

	if count != 3 {
		t.Fatalf("Expected 3 keys, got %d", count)
	}

	// List should return all keys ordered by scheme, key_identifier.
	keys, err = database.ListHomeNetworkKeys(ctx)
	if err != nil {
		t.Fatalf("ListHomeNetworkKeys failed: %s", err)
	}

	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys, got %d", len(keys))
	}
	// Order: A-0, A-1, B-0
	if keys[0].Scheme != "A" || keys[0].KeyIdentifier != 0 {
		t.Fatalf("Expected first key (A, 0), got (%s, %d)", keys[0].Scheme, keys[0].KeyIdentifier)
	}

	if keys[1].Scheme != "A" || keys[1].KeyIdentifier != 1 {
		t.Fatalf("Expected second key (A, 1), got (%s, %d)", keys[1].Scheme, keys[1].KeyIdentifier)
	}

	if keys[2].Scheme != "B" || keys[2].KeyIdentifier != 0 {
		t.Fatalf("Expected third key (B, 0), got (%s, %d)", keys[2].Scheme, keys[2].KeyIdentifier)
	}

	// Delete the rotation key.
	err = database.DeleteHomeNetworkKey(ctx, keys[1].ID)
	if err != nil {
		t.Fatalf("DeleteHomeNetworkKey failed: %s", err)
	}

	count, err = database.CountHomeNetworkKeys(ctx)
	if err != nil {
		t.Fatalf("CountHomeNetworkKeys failed: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected 2 keys after deletion, got %d", count)
	}
}

func TestCreateHomeNetworkKey_Duplicate(t *testing.T) {
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

	// Default key (A, 0) already exists. Try to create another (A, 0).
	dupKey := &db.HomeNetworkKey{
		KeyIdentifier: 0,
		Scheme:        "A",
		PrivateKey:    "0000000000000000000000000000000000000000000000000000000000000001",
	}

	err = database.CreateHomeNetworkKey(ctx, dupKey)
	if err == nil {
		t.Fatal("Expected error for duplicate key, got nil")
	}

	if err != db.ErrAlreadyExists {
		t.Fatalf("Expected ErrAlreadyExists, got: %s", err)
	}
}

func TestGetHomeNetworkKey_NotFound(t *testing.T) {
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

	_, err = database.GetHomeNetworkKey(ctx, 9999)
	if err == nil {
		t.Fatal("Expected error for non-existent key, got nil")
	}

	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got: %s", err)
	}
}

func TestGetHomeNetworkKeyBySchemeAndIdentifier_NotFound(t *testing.T) {
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

	_, err = database.GetHomeNetworkKeyBySchemeAndIdentifier(ctx, "B", 0)
	if err == nil {
		t.Fatal("Expected error for non-existent key, got nil")
	}

	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got: %s", err)
	}
}

func TestHomeNetworkKey_GetPublicKey_ProfileA(t *testing.T) {
	key := &db.HomeNetworkKey{
		Scheme:     "A",
		PrivateKey: "c53c22208b61860b06c62e5406a7b330c2b577aa5558981510d128247d38bd1d",
	}

	pub, err := key.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey failed: %s", err)
	}

	if len(pub) != 64 {
		t.Fatalf("Expected 64-char public key, got %d", len(pub))
	}
}

func TestHomeNetworkKey_GetPublicKey_ProfileB(t *testing.T) {
	key := &db.HomeNetworkKey{
		Scheme:     "B",
		PrivateKey: "f1ab1074477ebcce59b97460c83b4071db578ffab54ee4fbc76aeca38e4b7b01",
	}

	pub, err := key.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey failed: %s", err)
	}

	if len(pub) != 66 {
		t.Fatalf("Expected 66-char public key, got %d", len(pub))
	}
	// Compressed P-256 keys start with 02 or 03.
	if pub[0] != '0' || (pub[1] != '2' && pub[1] != '3') {
		t.Fatalf("Expected compressed P-256 key (starting with 02 or 03), got prefix %s", pub[:2])
	}
}
