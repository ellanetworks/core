// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func setupPKIDB(t *testing.T) *db.Database {
	t.Helper()

	ctx := context.Background()

	tmpDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(tmpDir, "ella.db"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = dbInstance.Close() })

	return dbInstance
}

func TestClusterNodeCert_UpsertListGetDelete(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	now := time.Now().Unix()

	a := &db.ClusterNodeCert{
		NodeID:      1,
		Fingerprint: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CertPEM:     "-----BEGIN CERTIFICATE-----\nMIIA\n-----END CERTIFICATE-----",
		AddedAt:     now,
	}
	b := &db.ClusterNodeCert{
		NodeID:      2,
		Fingerprint: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CertPEM:     "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
		AddedAt:     now,
	}

	if err := database.UpsertClusterNodeCert(ctx, a); err != nil {
		t.Fatalf("upsert a: %v", err)
	}

	if err := database.UpsertClusterNodeCert(ctx, b); err != nil {
		t.Fatalf("upsert b: %v", err)
	}

	rows, err := database.ListClusterNodeCerts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	got, err := database.GetClusterNodeCertByFingerprint(ctx, a.Fingerprint)
	if err != nil {
		t.Fatalf("get by fp: %v", err)
	}

	if got.NodeID != 1 {
		t.Fatalf("nodeID mismatch: got %d", got.NodeID)
	}

	// Re-pin (rotation): upsert with same nodeID, different fingerprint.
	rotated := &db.ClusterNodeCert{
		NodeID:      1,
		Fingerprint: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		CertPEM:     "-----BEGIN CERTIFICATE-----\nMIIC\n-----END CERTIFICATE-----",
		AddedAt:     now + 1,
	}

	if err := database.UpsertClusterNodeCert(ctx, rotated); err != nil {
		t.Fatalf("upsert rotated: %v", err)
	}

	if _, err := database.GetClusterNodeCertByFingerprint(ctx, a.Fingerprint); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("old fingerprint should be gone, got err=%v", err)
	}

	if _, err := database.GetClusterNodeCertByFingerprint(ctx, rotated.Fingerprint); err != nil {
		t.Fatalf("new fingerprint should be present: %v", err)
	}

	if err := database.DeleteClusterNodeCert(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	rows, _ = database.ListClusterNodeCerts(ctx)
	if len(rows) != 1 || rows[0].NodeID != 2 {
		t.Fatalf("after delete expected only node 2, got %+v", rows)
	}
}

func TestClusterJoinHMAC_InitIdempotent(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	if _, err := database.GetClusterJoinHMACKey(ctx); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected ErrNotFound before init, got %v", err)
	}

	first := []byte("0123456789abcdef0123456789abcdef")
	if err := database.InitClusterJoinHMACKey(ctx, first); err != nil {
		t.Fatalf("init: %v", err)
	}

	got, err := database.GetClusterJoinHMACKey(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if string(got) != string(first) {
		t.Fatalf("hmac mismatch")
	}

	// Idempotent: second init does not overwrite.
	if err := database.InitClusterJoinHMACKey(ctx, []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")); err != nil {
		t.Fatalf("init second: %v", err)
	}

	got2, _ := database.GetClusterJoinHMACKey(ctx)
	if string(got2) != string(first) {
		t.Fatalf("hmac was overwritten")
	}
}

func TestClusterJoinTokens_MintGetConsume(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	tok := &db.ClusterJoinToken{
		ID:         "tok-1",
		NodeID:     7,
		ClaimsJSON: `{"id":"tok-1"}`,
		ExpiresAt:  time.Now().Add(time.Hour).Unix(),
	}

	if err := database.MintJoinTokenRecord(ctx, tok); err != nil {
		t.Fatalf("mint: %v", err)
	}

	row, err := database.GetJoinToken(ctx, "tok-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if row.ConsumedAt != 0 {
		t.Fatalf("expected unconsumed token, got %d", row.ConsumedAt)
	}

	if err := database.ConsumeJoinToken(ctx, "tok-1", 7); err != nil {
		t.Fatalf("consume: %v", err)
	}

	if err := database.ConsumeJoinToken(ctx, "tok-1", 7); !errors.Is(err, db.ErrJoinTokenAlreadyConsumed) {
		t.Fatalf("second consume should fail with ErrJoinTokenAlreadyConsumed, got %v", err)
	}
}
