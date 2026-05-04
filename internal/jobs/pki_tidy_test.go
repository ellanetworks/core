// Copyright 2026 Ella Networks

package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestJoinTokenTidy_DeletesStale(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = database.Close() }()

	now := time.Now()

	stale := &db.ClusterJoinToken{
		ID:         "expired",
		NodeID:     2,
		ClaimsJSON: "{}",
		ExpiresAt:  now.Add(-time.Hour).Unix(),
	}

	live := &db.ClusterJoinToken{
		ID:         "live",
		NodeID:     2,
		ClaimsJSON: "{}",
		ExpiresAt:  now.Add(time.Hour).Unix(),
	}

	if err := database.MintJoinTokenRecord(ctx, stale); err != nil {
		t.Fatal(err)
	}

	if err := database.MintJoinTokenRecord(ctx, live); err != nil {
		t.Fatal(err)
	}

	if err := database.DeleteStaleJoinTokens(ctx, now); err != nil {
		t.Fatal(err)
	}

	if _, err := database.GetJoinToken(ctx, "expired"); err == nil {
		t.Fatal("expected stale token to be removed")
	}

	if _, err := database.GetJoinToken(ctx, "live"); err != nil {
		t.Fatalf("live token should remain: %v", err)
	}
}
