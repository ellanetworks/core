// Copyright 2026 Ella Networks

package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestPKITidy_DeletesExpiredRows(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = database.Close() }()

	now := time.Now()

	// Seed: one issued cert well in the past (should be purged), one
	// currently valid (should survive).
	if err := database.RecordIssuedCert(ctx, &db.ClusterIssuedCert{
		Serial:                  1,
		NodeID:                  2,
		NotAfter:                now.Add(-24 * time.Hour).Unix(),
		IntermediateFingerprint: "sha256:x",
		IssuedAt:                now.Add(-48 * time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := database.RecordIssuedCert(ctx, &db.ClusterIssuedCert{
		Serial:                  2,
		NodeID:                  2,
		NotAfter:                now.Add(24 * time.Hour).Unix(),
		IntermediateFingerprint: "sha256:x",
		IssuedAt:                now.Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	// One revoked cert whose purge window has passed.
	if err := database.InsertRevokedCert(ctx, &db.ClusterRevokedCert{
		Serial:     99,
		NodeID:     2,
		RevokedAt:  now.Add(-48 * time.Hour).Unix(),
		Reason:     "test",
		PurgeAfter: now.Add(-time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	// One revoked cert still in its window.
	if err := database.InsertRevokedCert(ctx, &db.ClusterRevokedCert{
		Serial:     100,
		NodeID:     3,
		RevokedAt:  now.Unix(),
		Reason:     "test",
		PurgeAfter: now.Add(24 * time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	// Run tidy.
	runPKITidyOnce(ctx, database)

	remaining, err := database.ListActiveIssuedCertsByNode(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(remaining) != 1 || remaining[0].Serial != 2 {
		t.Fatalf("expected only serial 2 active, got %+v", remaining)
	}

	revoked, err := database.ListRevokedCerts(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(revoked) != 1 || revoked[0].Serial != 100 {
		t.Fatalf("expected only serial 100 revoked after tidy, got %+v", revoked)
	}
}
