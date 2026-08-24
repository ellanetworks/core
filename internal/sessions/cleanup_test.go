// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sessions

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func newCleanupTestDB(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

func seedSessions(t *testing.T, database *db.Database) string {
	t.Helper()

	ctx := context.Background()

	userID, err := database.CreateUser(ctx, &db.User{
		Email:          "cleanup@example.com",
		RoleID:         db.RoleAdmin,
		HashedPassword: "not-a-real-hash",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now()

	for i, expiresAt := range []int64{now.Add(-time.Hour).Unix(), now.Add(time.Hour).Unix()} {
		if err := database.CreateSession(ctx, &db.Session{
			UserID:    userID,
			TokenHash: []byte{byte(i)},
			CreatedAt: now.Add(-2 * time.Hour).Unix(),
			ExpiresAt: expiresAt,
		}); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	return userID
}

func TestCleanUpDeletesExpiredSessionsWithoutRaft(t *testing.T) {
	database := newCleanupTestDB(t)
	userID := seedSessions(t, database)

	if !database.IsLeader() {
		t.Fatal("a database without raft must report itself leader")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	go func() {
		CleanUp(ctx, database)
		close(done)
	}()

	deadline := time.Now().Add(10 * time.Second)

	for countSessions(t, database, userID) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("cleanup did not delete the expired session on a non-clustered node")
		}

		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done
}

func countSessions(t *testing.T, database *db.Database, userID string) int {
	t.Helper()

	n, err := database.CountSessionsByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}

	return n
}
