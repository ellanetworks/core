// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestSessionsEndToEnd(t *testing.T) {
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

	user := &db.User{
		Email:          "testuser@example.com",
		HashedPassword: "afewfawe12321",
	}

	userID, err := database.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	expiresAt := time.Now().Add(1 * time.Hour)

	now := time.Now()

	session := &db.Session{
		UserID:    userID,
		TokenHash: make([]byte, 32),
		CreatedAt: now.Unix(),
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

	if retrievedSession.CreatedAt != now.Unix() {
		t.Fatalf("The createdAt time from the database doesn't match the expected value")
	}
}

func TestDeleteSessionByTokenHash(t *testing.T) {
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

	user := &db.User{
		Email:          "testuser@example.com",
		HashedPassword: "afewfawe12321",
	}

	userID, err := database.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	expiresAt := time.Now().Add(1 * time.Hour)

	session := &db.Session{
		UserID:    userID,
		TokenHash: make([]byte, 32),
		CreatedAt: time.Now().Unix(),
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

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	user1 := &db.User{
		Email:          "testuser1@example.com",
		HashedPassword: "afewfawe12321",
	}

	userID1, err := database.CreateUser(context.Background(), user1)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	user2 := &db.User{
		Email:          "testuser2@example.com",
		HashedPassword: "afewfawe12321",
	}

	userID2, err := database.CreateUser(context.Background(), user2)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	now := time.Now()

	expiredSession := &db.Session{
		UserID:    userID1,
		TokenHash: []byte{1, 2, 3, 4, 5},
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(-1 * time.Hour).Unix(), // already expired
	}

	validSession := &db.Session{
		UserID:    userID2,
		TokenHash: []byte{6, 7, 8, 9, 10},
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(1 * time.Hour).Unix(),
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

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
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

	for i := range totalSessions {
		user := &db.User{
			Email:          fmt.Sprintf("testuser%d@example.com", i),
			HashedPassword: "afewfawe12321",
		}

		userID, err := database.CreateUser(context.Background(), user)
		if err != nil {
			t.Fatalf("Couldn't complete CreateUser: %s", err)
		}

		now := time.Now()
		session := &db.Session{
			UserID:    userID,
			TokenHash: []byte{byte(i)},
			CreatedAt: now.Unix(),
		}

		if i < expiredSessions {
			session.ExpiresAt = now.Add(-5 * time.Minute).Unix() // already expired
		} else {
			session.ExpiresAt = now.Add(5 * time.Minute).Unix() // valid
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

func TestDeleteAllSessionsForUser(t *testing.T) {
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

	user1 := &db.User{
		Email:          "user1@example.com",
		HashedPassword: "hash1",
	}

	userID1, err := database.CreateUser(context.Background(), user1)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	user2 := &db.User{
		Email:          "user2@example.com",
		HashedPassword: "hash2",
	}

	userID2, err := database.CreateUser(context.Background(), user2)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	now := time.Now()
	expiresAt := now.Add(1 * time.Hour).Unix()

	// Create 3 sessions for user1 and 1 for user2
	for i := range 3 {
		session := &db.Session{
			UserID:    userID1,
			TokenHash: []byte{byte(i + 1)},
			CreatedAt: now.Unix(),
			ExpiresAt: expiresAt,
		}

		_, err = database.CreateSession(context.Background(), session)
		if err != nil {
			t.Fatalf("Couldn't complete CreateSession: %s", err)
		}
	}

	user2Session := &db.Session{
		UserID:    userID2,
		TokenHash: []byte{10},
		CreatedAt: now.Unix(),
		ExpiresAt: expiresAt,
	}

	_, err = database.CreateSession(context.Background(), user2Session)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSession: %s", err)
	}

	err = database.DeleteAllSessionsForUser(context.Background(), userID1)
	if err != nil {
		t.Fatalf("Couldn't complete DeleteAllSessionsForUser: %s", err)
	}

	// All user1 sessions should be gone
	for i := range 3 {
		_, err := database.GetSessionByTokenHash(context.Background(), []byte{byte(i + 1)})
		if err == nil {
			t.Fatalf("Expected error when retrieving deleted session %d for user1", i)
		}
	}

	// User2 session should still exist
	retrieved, err := database.GetSessionByTokenHash(context.Background(), user2Session.TokenHash)
	if err != nil {
		t.Fatalf("Couldn't retrieve user2 session after deleting user1 sessions: %s", err)
	}

	if retrieved == nil {
		t.Fatalf("Expected user2 session to still exist")
	}
}

func TestDeleteAllSessions(t *testing.T) {
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

	user1 := &db.User{
		Email:          "user1@example.com",
		HashedPassword: "hash1",
	}

	userID1, err := database.CreateUser(context.Background(), user1)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	user2 := &db.User{
		Email:          "user2@example.com",
		HashedPassword: "hash2",
	}

	userID2, err := database.CreateUser(context.Background(), user2)
	if err != nil {
		t.Fatalf("Couldn't complete CreateUser: %s", err)
	}

	now := time.Now()
	expiresAt := now.Add(1 * time.Hour).Unix()

	sessions := []*db.Session{
		{UserID: userID1, TokenHash: []byte{1}, CreatedAt: now.Unix(), ExpiresAt: expiresAt},
		{UserID: userID1, TokenHash: []byte{2}, CreatedAt: now.Unix(), ExpiresAt: expiresAt},
		{UserID: userID2, TokenHash: []byte{3}, CreatedAt: now.Unix(), ExpiresAt: expiresAt},
	}

	for _, s := range sessions {
		_, err = database.CreateSession(context.Background(), s)
		if err != nil {
			t.Fatalf("Couldn't complete CreateSession: %s", err)
		}
	}

	err = database.DeleteAllSessions(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete DeleteAllSessions: %s", err)
	}

	// All sessions across all users should be gone
	for _, s := range sessions {
		_, err := database.GetSessionByTokenHash(context.Background(), s.TokenHash)
		if err == nil {
			t.Fatalf("Expected error when retrieving deleted session with hash %v", s.TokenHash)
		}
	}
}
