// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/models"
)

func TestSessionManager_CreateSession(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestImmediate,
		Method:      MethodCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
}

func TestSessionManager_CreateSession_DefaultMethod(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestImmediate,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	session, err := database.GetPositioningSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.Method != "cell_id" {
		t.Errorf("expected default method cell_id, got %s", session.Method)
	}
}

func TestSessionManager_CreateSession_PersonicSessionType(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestPeriodic,
		Method:      MethodECID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	session, err := database.GetPositioningSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.SessionType != 1 {
		t.Errorf("expected session type 1 (periodic), got %d", session.SessionType)
	}
}

func TestSessionManager_CreateSession_TriggeredSessionType(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestTriggered,
		Method:      MethodECID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	session, err := database.GetPositioningSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.SessionType != 2 {
		t.Errorf("expected session type 2 (triggered), got %d", session.SessionType)
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestImmediate,
		Method:      MethodCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	session, err := smgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.SUPI != "imsi-123456789012345" {
		t.Errorf("expected SUPI imsi-123456789012345, got %s", session.SUPI)
	}

	if session.Status != int(SessionStatusActive) {
		t.Errorf("expected status %d, got %d", SessionStatusActive, session.Status)
	}
}

func TestSessionManager_CompleteSession(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestImmediate,
		Method:      MethodCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	err = smgr.CompleteSession(context.Background(), sessionID, &models.LocationResult{
		SUPI:  "imsi-123456789012345",
		Shape: models.GADCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	session, err := smgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.Status != int(SessionStatusCompleted) {
		t.Errorf("expected status %d, got %d", SessionStatusCompleted, session.Status)
	}
}

func TestSessionManager_FailSession(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestImmediate,
		Method:      MethodCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	err = smgr.FailSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	session, err := smgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.Status != int(SessionStatusFailed) {
		t.Errorf("expected status %d, got %d", SessionStatusFailed, session.Status)
	}
}

func TestSessionManager_CancelSession(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	sessionID, err := smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: "imsi-123456789012345",

		RequestType: RequestImmediate,
		Method:      MethodCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	err = smgr.CancelSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	session, err := smgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.Status != int(SessionStatusCancelled) {
		t.Errorf("expected status %d, got %d", SessionStatusCancelled, session.Status)
	}
}

func TestSessionManager_ListSessionsBySupi(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	supi := "imsi-123456789012345"

	_, err = smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: supi,

		RequestType: RequestImmediate,
		Method:      MethodCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, err = smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: supi,

		RequestType: RequestImmediate,
		Method:      MethodECID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	sessions, err := smgr.ListSessionsBySupi(context.Background(), supi)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestSessionManager_GetActiveSessionBySupi(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Logf("closing database: %v", err)
		}
	}()

	smgr := NewSessionManager(database)

	supi := "imsi-123456789012345"

	_, err = smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: supi,

		RequestType: RequestImmediate,
		Method:      MethodCellID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, err = smgr.CreateSession(context.Background(), CreateSessionParams{
		SUPI: supi,

		RequestType: RequestImmediate,
		Method:      MethodECID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	activeSession, err := smgr.GetActiveSessionBySupi(context.Background(), supi)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if activeSession == nil {
		t.Fatal("expected active session, got nil")
	}
}

func TestDefaultMethodForRequest(t *testing.T) {
	if got := DefaultMethodForRequest(RequestImmediate); got != MethodCellID {
		t.Errorf("DefaultMethodForRequest(immediate) = %s, want %s", got, MethodCellID)
	}

	if got := DefaultMethodForRequest(RequestPeriodic); got != MethodCellID {
		t.Errorf("DefaultMethodForRequest(periodic) = %s, want %s", got, MethodCellID)
	}

	if got := DefaultMethodForRequest(RequestTriggered); got != MethodCellID {
		t.Errorf("DefaultMethodForRequest(triggered) = %s, want %s", got, MethodCellID)
	}

	if got := DefaultMethodForRequest(RequestCancel); got != MethodCellID {
		t.Errorf("DefaultMethodForRequest(cancel) = %s, want %s", got, MethodCellID)
	}
}

func TestSessionTypeFromRequest(t *testing.T) {
	if got := SessionTypeFromRequest(RequestImmediate); got != SessionTypeImmediate {
		t.Errorf("SessionTypeFromRequest(immediate) = %d, want %d", got, SessionTypeImmediate)
	}

	if got := SessionTypeFromRequest(RequestPeriodic); got != SessionTypePeriodic {
		t.Errorf("SessionTypeFromRequest(periodic) = %d, want %d", got, SessionTypePeriodic)
	}

	if got := SessionTypeFromRequest(RequestTriggered); got != SessionTypeTriggered {
		t.Errorf("SessionTypeFromRequest(triggered) = %d, want %d", got, SessionTypeTriggered)
	}

	if got := SessionTypeFromRequest(RequestCancel); got != SessionTypeImmediate {
		t.Errorf("SessionTypeFromRequest(cancel) = %d, want %d", got, SessionTypeImmediate)
	}

	if got := SessionTypeFromRequest(RequestType("unknown")); got != SessionTypeImmediate {
		t.Errorf("SessionTypeFromRequest(unknown) = %d, want %d", got, SessionTypeImmediate)
	}
}
