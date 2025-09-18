// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestSessionsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	expiresAt := time.Now().Add(1 * time.Hour)

	session := &db.Session{
		UserID:    1,
		TokenHash: make([]byte, 32),
		ExpiresAt: expiresAt.Unix(),
	}

	_, err = database.CreateSession(context.Background(), session)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSession: %s", err)
	}

	retrievedSession, err := database.GetSessionByTokenHash(context.Background(), session.TokenHash)
	if err != nil {
		t.Fatalf("Couldn't complete GetSessionByTokenHash: %s", err)
	}

	if retrievedSession.UserID != session.UserID {
		t.Fatalf("The user ID from the database doesn't match the expected value")
	}

	retrievedExpiresAt := time.Unix(retrievedSession.ExpiresAt, 0)

	if retrievedExpiresAt.Unix() != expiresAt.Unix() {
		t.Fatalf("The expiry time from the database doesn't match the expected value")
	}

	if len(retrievedSession.TokenHash) != len(session.TokenHash) {
		t.Fatalf("The token hash length from the database doesn't match the expected value")
	}

	if retrievedSession.CreatedAt == 0 {
		t.Fatalf("The createdAt time from the database is empty")
	}

	createdAt := time.Unix(retrievedSession.CreatedAt, 0)

	if time.Since(createdAt) > 5*time.Minute {
		t.Fatalf("The createdAt time from the database is not recent")
	}
}

func TestDeleteSessionByTokenHash(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	expiresAt := time.Now().Add(1 * time.Hour)

	session := &db.Session{
		UserID:    1,
		TokenHash: make([]byte, 32),
		ExpiresAt: expiresAt.Unix(),
	}

	_, err = database.CreateSession(context.Background(), session)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSession: %s", err)
	}

	err = database.DeleteSessionByTokenHash(context.Background(), session.TokenHash)
	if err != nil {
		t.Fatalf("Couldn't complete DeleteSessionByTokenHash: %s", err)
	}

	retrievedSession, err := database.GetSessionByTokenHash(context.Background(), session.TokenHash)
	if err == nil {
		t.Fatalf("Expected error when retrieving deleted session, got nil")
	}

	if retrievedSession != nil {
		t.Fatalf("Expected retrieved session to be nil after deletion")
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	expiredSession := &db.Session{
		UserID:    1,
		TokenHash: []byte{1, 2, 3, 4, 5},
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(), // already expired
	}

	validSession := &db.Session{
		UserID:    2,
		TokenHash: []byte{6, 7, 8, 9, 10},
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}

	_, err = database.CreateSession(context.Background(), expiredSession)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSession for expired session: %s", err)
	}

	_, err = database.CreateSession(context.Background(), validSession)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSession for valid session: %s", err)
	}

	numDeleted, err := database.DeleteExpiredSessions(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete DeleteExpiredSessions: %s", err)
	}

	if numDeleted != 1 {
		t.Fatalf("Expected 1 expired session to be deleted, got %d", numDeleted)
	}

	retrievedExpiredSession, err := database.GetSessionByTokenHash(context.Background(), expiredSession.TokenHash)
	if err == nil {
		t.Fatalf("Expected error when retrieving deleted expired session, got nil")
	}

	if retrievedExpiredSession != nil {
		t.Fatalf("Expected retrieved expired session to be nil after deletion")
	}

	retrievedValidSession, err := database.GetSessionByTokenHash(context.Background(), validSession.TokenHash)
	if err != nil {
		t.Fatalf("Couldn't retrieve valid session after deleting expired sessions: %s", err)
	}

	if retrievedValidSession == nil {
		t.Fatalf("Expected valid session to still exist after deleting expired sessions")
	}
}

// Create 100 sessions, half expired, half valid, and ensure that
// DeleteExpiredSessions deletes the correct number of expired sessions.
func TestDeleteManyExpiredSessions(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	totalSessions := 100
	expiredSessions := totalSessions / 2

	for i := 0; i < totalSessions; i++ {
		session := &db.Session{
			UserID:    i,
			TokenHash: []byte{byte(i)},
		}

		if i < expiredSessions {
			session.ExpiresAt = time.Now().Add(-5 * time.Minute).Unix() // already expired
		} else {
			session.ExpiresAt = time.Now().Add(5 * time.Minute).Unix() // valid
		}

		_, err = database.CreateSession(context.Background(), session)
		if err != nil {
			t.Fatalf("Couldn't complete CreateSession for session %d: %s", i, err)
		}
	}

	numDeleted, err := database.DeleteExpiredSessions(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete DeleteExpiredSessions: %s", err)
	}

	if numDeleted != expiredSessions {
		t.Fatalf("Expected %d expired sessions to be deleted, got %d", expiredSessions, numDeleted)
	}

	// Verify that all expired sessions are deleted and valid sessions remain
	for i := 0; i < totalSessions; i++ {
		tokenHash := []byte{byte(i)}
		retrievedSession, err := database.GetSessionByTokenHash(context.Background(), tokenHash)

		if i < expiredSessions {
			if err == nil {
				t.Fatalf("Expected error when retrieving deleted expired session %d, got nil", i)
			}
			if retrievedSession != nil {
				t.Fatalf("Expected retrieved expired session %d to be nil after deletion", i)
			}
		} else {
			if err != nil {
				t.Fatalf("Couldn't retrieve valid session %d after deleting expired sessions: %s", i, err)
			}
			if retrievedSession == nil {
				t.Fatalf("Expected valid session %d to still exist after deleting expired sessions", i)
			}
		}
	}
}
