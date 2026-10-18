// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func expectOneRaftEntry(t *testing.T, database *db.Database, what string, write func() error) {
	t.Helper()

	before := database.RaftAppliedIndex()

	if err := write(); err != nil {
		t.Fatalf("%s: %v", what, err)
	}

	if got := database.RaftAppliedIndex() - before; got != 1 {
		t.Fatalf("%s applied %d Raft entries, want 1: the write must replicate", what, got)
	}
}

func awaitWakeup(t *testing.T, wakeup <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-wakeup:
	case <-time.After(time.Second):
		t.Fatalf("no changefeed wakeup after %s", what)
	}
}

func TestRotateJWTSecretReplicatesSecretAndSessionWipe(t *testing.T) {
	ctx := context.Background()
	database := setupRaftTestDB(t)

	userID, err := database.CreateUser(ctx, &db.User{Email: "rotate@example.com", HashedPassword: "x"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	tokenHash := bytes.Repeat([]byte{7}, 32)

	if err := database.CreateSession(ctx, &db.Session{
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	wakeup, stop := database.Changefeed().Wakeup(db.TopicJWTSecret)
	defer stop()

	newSecret := bytes.Repeat([]byte{9}, 32)

	expectOneRaftEntry(t, database, "RotateJWTSecret", func() error {
		return database.RotateJWTSecret(ctx, newSecret)
	})

	got, err := database.GetJWTSecret(ctx)
	if err != nil {
		t.Fatalf("GetJWTSecret: %v", err)
	}

	if !bytes.Equal(got, newSecret) {
		t.Fatalf("secret after rotation = %x, want %x", got, newSecret)
	}

	if _, err := database.GetSessionByTokenHash(ctx, tokenHash); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("session after rotation: err = %v, want ErrNotFound", err)
	}

	awaitWakeup(t, wakeup, "RotateJWTSecret")
}

func TestCellPositionWritesReplicate(t *testing.T) {
	ctx := context.Background()
	database := setupRaftTestDB(t)

	position := &db.CellPosition{
		RAT:          db.RATNR,
		Mcc:          "001",
		Mnc:          "01",
		CellIdentity: "00000A001",
		Latitude:     45.5,
		Longitude:    -73.5,
	}

	expectOneRaftEntry(t, database, "CreateCellPosition", func() error {
		return database.CreateCellPosition(ctx, position)
	})

	duplicate := *position
	duplicate.ID = ""

	before := database.RaftAppliedIndex()

	if err := database.CreateCellPosition(ctx, &duplicate); !errors.Is(err, db.ErrAlreadyExists) {
		t.Fatalf("duplicate CreateCellPosition: err = %v, want ErrAlreadyExists", err)
	}

	if database.RaftAppliedIndex() != before {
		t.Fatalf("a rejected duplicate must not reach the Raft log")
	}

	position.Latitude = 46.5

	expectOneRaftEntry(t, database, "UpdateCellPosition", func() error {
		return database.UpdateCellPosition(ctx, position)
	})

	got, err := database.GetCellPosition(ctx, position.ID)
	if err != nil {
		t.Fatalf("GetCellPosition: %v", err)
	}

	if got.Latitude != 46.5 || got.CellIdentity != "00000a001" {
		t.Fatalf("cell position after update = %+v", got)
	}

	expectOneRaftEntry(t, database, "DeleteCellPosition", func() error {
		return database.DeleteCellPosition(ctx, position.ID)
	})

	if err := database.UpdateCellPosition(ctx, position); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("UpdateCellPosition after delete: err = %v, want ErrNotFound", err)
	}

	if err := database.DeleteCellPosition(ctx, position.ID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("DeleteCellPosition after delete: err = %v, want ErrNotFound", err)
	}
}

func TestReopenWakesEveryChangefeedSubscriber(t *testing.T) {
	database := setupTestDB(t)

	wakeup, stop := database.Changefeed().Wakeup(db.TopicJWTSecret)
	defer stop()

	if err := database.Reopen(context.Background()); err != nil {
		t.Fatalf("Reopen: %v", err)
	}

	awaitWakeup(t, wakeup, "Reopen")
}
