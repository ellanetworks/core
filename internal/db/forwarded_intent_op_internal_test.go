// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestApplyForwardedOperation_IntentOpReturnsResult(t *testing.T) {
	database := newAtomicTestDB(t)
	ctx := context.Background()

	var (
		liveToken    = []byte("live-session-token-hash")
		expiredToken = []byte("expired-session-token-hash")
	)

	userID, err := database.CreateUser(ctx, &User{
		Email:          "sessions@ellanetworks.com",
		RoleID:         RoleAdmin,
		HashedPassword: "not-a-real-hash",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	now := time.Now()

	seedSession(t, ctx, database, userID, liveToken, now.Add(time.Hour))
	seedSession(t, ctx, database, userID, expiredToken, now.Add(-time.Hour))

	payload, marshalErr := json.Marshal(&int64Payload{Value: now.Unix()})
	if marshalErr != nil {
		t.Fatalf("marshal payload: %v", marshalErr)
	}

	result, err := database.ApplyForwardedOperation("DeleteExpiredSessions", payload)
	if err != nil {
		t.Fatalf("ApplyForwardedOperation(DeleteExpiredSessions): %v", err)
	}

	if result.Index == 0 {
		t.Error("forwarded intent op returned index 0; a follower would report an uncommitted write as durable")
	}

	deleted, err := narrowResult[int]("DeleteExpiredSessions", result.Value)
	if err != nil {
		t.Fatalf("result value did not survive the forwarding boundary: %v", err)
	}

	if deleted != 1 {
		t.Errorf("deleted sessions = %d, want 1 (only the expired session)", deleted)
	}

	if _, err := database.GetSessionByTokenHash(ctx, expiredToken); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session still present after forwarded op: err = %v", err)
	}

	if _, err := database.GetSessionByTokenHash(ctx, liveToken); err != nil {
		t.Errorf("live session removed by forwarded op: %v", err)
	}
}

func TestApplyForwardedOperation_UnknownOpIsRejected(t *testing.T) {
	database := newAtomicTestDB(t)

	_, err := database.ApplyForwardedOperation("ThisOperationDoesNotExist", json.RawMessage(`{}`))
	if !errors.Is(err, ErrUnknownOperation) {
		t.Fatalf("error = %v, want ErrUnknownOperation", err)
	}
}

func seedSession(t *testing.T, ctx context.Context, database *Database, userID string, tokenHash []byte, expiresAt time.Time) {
	t.Helper()

	if err := database.CreateSession(ctx, &Session{
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expiresAt.Unix(),
	}); err != nil {
		t.Fatalf("seed session %q: %v", tokenHash, err)
	}
}
