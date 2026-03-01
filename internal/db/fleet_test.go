// Copyright 2026 Ella Networks

package db_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetFleet_Default(t *testing.T) {
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

	fleet, err := database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if fleet == nil {
		t.Fatalf("Fleet should not be nil after initialization")
	}

	if fleet.Enabled != true {
		t.Fatalf("Fleet should be enabled by default")
	}

	if len(fleet.PrivateKey) != 0 {
		t.Fatalf("Fleet private key should be empty by default, got %d bytes", len(fleet.PrivateKey))
	}

	if len(fleet.Certificate) != 0 {
		t.Fatalf("Fleet certificate should be empty by default, got %d bytes", len(fleet.Certificate))
	}

	if len(fleet.CACertificate) != 0 {
		t.Fatalf("Fleet CA certificate should be empty by default, got %d bytes", len(fleet.CACertificate))
	}

	if fleet.ConfigRevision != 0 {
		t.Fatalf("Fleet config revision should be 0 by default, got %d", fleet.ConfigRevision)
	}
}

func TestUpdateFleetKey(t *testing.T) {
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

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't generate key: %s", err)
	}

	err = database.UpdateFleetKey(context.Background(), key)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetKey: %s", err)
	}

	fleet, err := database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if len(fleet.PrivateKey) == 0 {
		t.Fatalf("Fleet private key should not be empty after update")
	}

	parsedKey, err := x509.ParseECPrivateKey(fleet.PrivateKey)
	if err != nil {
		t.Fatalf("Couldn't parse stored private key: %s", err)
	}

	if !parsedKey.PublicKey.Equal(&key.PublicKey) {
		t.Fatalf("Stored key doesn't match original key")
	}
}

func TestUpdateFleetCredentials(t *testing.T) {
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

	cert := []byte("-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----")
	caCert := []byte("-----BEGIN CERTIFICATE-----\ntest-ca-cert\n-----END CERTIFICATE-----")

	err = database.UpdateFleetCredentials(context.Background(), cert, caCert)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetCredentials: %s", err)
	}

	fleet, err := database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if !bytes.Equal(fleet.Certificate, cert) {
		t.Fatalf("Fleet certificate doesn't match: got %q, want %q", fleet.Certificate, cert)
	}

	if !bytes.Equal(fleet.CACertificate, caCert) {
		t.Fatalf("Fleet CA certificate doesn't match: got %q, want %q", fleet.CACertificate, caCert)
	}
}

func TestLoadOrGenerateFleetKey_Generates(t *testing.T) {
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

	// With a fresh database, the key should be empty, so LoadOrGenerateFleetKey
	// should generate a new one.
	key, err := database.LoadOrGenerateFleetKey(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete LoadOrGenerateFleetKey: %s", err)
	}

	if key == nil {
		t.Fatalf("Expected a generated key, got nil")
	}

	if key.Curve != elliptic.P256() {
		t.Fatalf("Expected P-256 curve, got %v", key.Curve.Params().Name)
	}

	// Verify the key was persisted
	fleet, err := database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if len(fleet.PrivateKey) == 0 {
		t.Fatalf("Expected key to be persisted in database")
	}
}

func TestLoadOrGenerateFleetKey_Loads(t *testing.T) {
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

	// Pre-store a key
	originalKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't generate key: %s", err)
	}

	err = database.UpdateFleetKey(context.Background(), originalKey)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetKey: %s", err)
	}

	// LoadOrGenerateFleetKey should load the existing key, not generate a new one
	loadedKey, err := database.LoadOrGenerateFleetKey(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete LoadOrGenerateFleetKey: %s", err)
	}

	if !loadedKey.PublicKey.Equal(&originalKey.PublicKey) {
		t.Fatalf("Loaded key doesn't match original key")
	}
}

func TestLoadOrGenerateFleetKey_Idempotent(t *testing.T) {
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

	// Call twice — should return the same key both times
	key1, err := database.LoadOrGenerateFleetKey(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete first LoadOrGenerateFleetKey: %s", err)
	}

	key2, err := database.LoadOrGenerateFleetKey(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete second LoadOrGenerateFleetKey: %s", err)
	}

	if !key1.PublicKey.Equal(&key2.PublicKey) {
		t.Fatalf("Expected same key on consecutive calls, but they differ")
	}
}

func TestUpdateFleetCredentials_Overwrite(t *testing.T) {
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

	cert1 := []byte("cert-1")
	caCert1 := []byte("ca-cert-1")

	err = database.UpdateFleetCredentials(context.Background(), cert1, caCert1)
	if err != nil {
		t.Fatalf("Couldn't complete first UpdateFleetCredentials: %s", err)
	}

	cert2 := []byte("cert-2")
	caCert2 := []byte("ca-cert-2")

	err = database.UpdateFleetCredentials(context.Background(), cert2, caCert2)
	if err != nil {
		t.Fatalf("Couldn't complete second UpdateFleetCredentials: %s", err)
	}

	fleet, err := database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if !bytes.Equal(fleet.Certificate, cert2) {
		t.Fatalf("Fleet certificate should be overwritten: got %q, want %q", fleet.Certificate, cert2)
	}

	if !bytes.Equal(fleet.CACertificate, caCert2) {
		t.Fatalf("Fleet CA certificate should be overwritten: got %q, want %q", fleet.CACertificate, caCert2)
	}
}

func TestFleet_RestartDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	// Store a key and credentials
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't generate key: %s", err)
	}

	err = database.UpdateFleetKey(context.Background(), key)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetKey: %s", err)
	}

	cert := []byte("persisted-cert")
	caCert := []byte("persisted-ca-cert")

	err = database.UpdateFleetCredentials(context.Background(), cert, caCert)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetCredentials: %s", err)
	}

	// Close and reopen the database
	err = database.Close()
	if err != nil {
		t.Fatalf("Couldn't complete Close: %s", err)
	}

	database2, err := db.NewDatabase(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Couldn't complete second NewDatabase: %s", err)
	}

	defer func() {
		if err := database2.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	// Verify all data survived the restart
	fleet, err := database2.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if !bytes.Equal(fleet.Certificate, cert) {
		t.Fatalf("Fleet certificate didn't survive restart: got %q, want %q", fleet.Certificate, cert)
	}

	if !bytes.Equal(fleet.CACertificate, caCert) {
		t.Fatalf("Fleet CA certificate didn't survive restart: got %q, want %q", fleet.CACertificate, caCert)
	}

	loadedKey, err := database2.LoadOrGenerateFleetKey(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete LoadOrGenerateFleetKey: %s", err)
	}

	if !loadedKey.PublicKey.Equal(&key.PublicKey) {
		t.Fatalf("Fleet key didn't survive restart")
	}
}

func TestUpdateFleetConfigRevision(t *testing.T) {
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

	// Default revision should be 0
	fleet, err := database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if fleet.ConfigRevision != 0 {
		t.Fatalf("Expected default config revision 0, got %d", fleet.ConfigRevision)
	}

	// Update to revision 5
	err = database.UpdateFleetConfigRevision(context.Background(), 5)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetConfigRevision: %s", err)
	}

	fleet, err = database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if fleet.ConfigRevision != 5 {
		t.Fatalf("Expected config revision 5, got %d", fleet.ConfigRevision)
	}

	// Update to a higher revision
	err = database.UpdateFleetConfigRevision(context.Background(), 42)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetConfigRevision: %s", err)
	}

	fleet, err = database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if fleet.ConfigRevision != 42 {
		t.Fatalf("Expected config revision 42, got %d", fleet.ConfigRevision)
	}
}

func TestClearFleetCredentials_ResetsConfigRevision(t *testing.T) {
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

	// Set credentials and a config revision
	cert := []byte("test-cert")
	caCert := []byte("test-ca-cert")

	err = database.UpdateFleetCredentials(context.Background(), cert, caCert)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetCredentials: %s", err)
	}

	err = database.UpdateFleetConfigRevision(context.Background(), 10)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetConfigRevision: %s", err)
	}

	// Clear credentials should reset config revision to 0
	err = database.ClearFleetCredentials(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete ClearFleetCredentials: %s", err)
	}

	fleet, err := database.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if fleet.ConfigRevision != 0 {
		t.Fatalf("Expected config revision to be reset to 0 after clearing credentials, got %d", fleet.ConfigRevision)
	}

	if len(fleet.Certificate) != 0 {
		t.Fatalf("Expected certificate to be empty after clearing credentials")
	}

	if len(fleet.CACertificate) != 0 {
		t.Fatalf("Expected CA certificate to be empty after clearing credentials")
	}
}

func TestFleetConfigRevision_SurvivesRestart(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	err = database.UpdateFleetConfigRevision(context.Background(), 99)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateFleetConfigRevision: %s", err)
	}

	err = database.Close()
	if err != nil {
		t.Fatalf("Couldn't complete Close: %s", err)
	}

	// Reopen the database
	database2, err := db.NewDatabase(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Couldn't complete second NewDatabase: %s", err)
	}

	defer func() {
		if err := database2.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	fleet, err := database2.GetFleet(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete GetFleet: %s", err)
	}

	if fleet.ConfigRevision != 99 {
		t.Fatalf("Expected config revision 99 after restart, got %d", fleet.ConfigRevision)
	}
}
