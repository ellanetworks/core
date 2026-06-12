// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// SessionManager handles the lifecycle of positioning sessions.
type SessionManager struct {
	db *db.Database
	mu sync.Mutex
}

// NewSessionManager creates a new session manager backed by the given database.
func NewSessionManager(d *db.Database) *SessionManager {
	return &SessionManager{db: d}
}

// CreateSessionParams holds the parameters for creating a positioning session.
type CreateSessionParams struct {
	SUPI              string
	AMFID             string
	RequestType       RequestType
	Method            PositioningMethod
	QoSResponseTimeMs *int
	QOSHAccuracyM     *int
}

// CreateSession creates a new positioning session and returns its ID.
func (m *SessionManager) CreateSession(ctx context.Context, params CreateSessionParams) (string, error) {
	method := params.Method
	if method == "" {
		method = DefaultMethodForRequest(params.RequestType)
	}

	s := &db.PositioningSession{
		SUPI:              params.SUPI,
		AMFID:             params.AMFID,
		SessionType:       int(SessionTypeFromRequest(params.RequestType)),
		Method:            string(method),
		QoSResponseTimeMs: params.QoSResponseTimeMs,
		QOSHAccuracyM:     params.QOSHAccuracyM,
		Status:            int(SessionStatusActive),
	}

	if err := m.db.CreatePositioningSession(ctx, s); err != nil {
		return "", fmt.Errorf("create positioning session: %w", err)
	}

	logger.LmfLog.Info("positioning session created",
		zap.String("session_id", s.ID),
		zap.String("supi", params.SUPI),
		zap.String("method", string(params.Method)),
	)

	return s.ID, nil
}

// CreateLPPSession creates a positioning session and initializes the LPP state
// machine for AGNSS positioning. It returns the LPP session handle and the
// positioning session ID. The caller must wire the LPP transport functions
// via session.SetTransport() before calling session.StartSession().
func (m *SessionManager) CreateLPPSession(ctx context.Context, params CreateSessionParams) (*lpp.Session, error) {
	sessionID, err := m.CreateSession(ctx, params)
	if err != nil {
		return nil, err
	}

	session := lpp.NewSession(params.SUPI, sessionID, string(params.Method))

	logger.LmfLog.Info("LPP session created",
		zap.String("session_id", sessionID),
		zap.String("supi", params.SUPI),
		zap.String("method", string(params.Method)),
	)

	return session, nil
}

// CompleteSession marks a session as completed with a location result.
func (m *SessionManager) CompleteSession(ctx context.Context, sessionID string, result *models.LocationResult) error {
	if result == nil {
		return fmt.Errorf("nil result")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal location result: %w", err)
	}

	resultStr := string(resultJSON)

	if err := m.db.UpdatePositioningSessionStatus(ctx, sessionID, int(SessionStatusCompleted), &resultStr); err != nil {
		return fmt.Errorf("complete session %s: %w", sessionID, err)
	}

	logger.LmfLog.Info("positioning session completed",
		zap.String("session_id", sessionID),
	)

	return nil
}

// FailSession marks a session as failed.
func (m *SessionManager) FailSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.db.UpdatePositioningSessionStatus(ctx, sessionID, int(SessionStatusFailed), nil); err != nil {
		return fmt.Errorf("fail session %s: %w", sessionID, err)
	}

	logger.LmfLog.Warn("positioning session failed",
		zap.String("session_id", sessionID),
	)

	return nil
}

// CancelSession marks a session as cancelled.
func (m *SessionManager) CancelSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.db.UpdatePositioningSessionStatus(ctx, sessionID, int(SessionStatusCancelled), nil); err != nil {
		return fmt.Errorf("cancel session %s: %w", sessionID, err)
	}

	logger.LmfLog.Info("positioning session cancelled",
		zap.String("session_id", sessionID),
	)

	return nil
}

// GetSession retrieves a session by ID, including its result if available.
func (m *SessionManager) GetSession(ctx context.Context, sessionID string) (*db.PositioningSession, error) {
	s, err := m.db.GetPositioningSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session %s: %w", sessionID, err)
	}

	return s, nil
}

// ListSessionsBySupi returns all sessions for a given SUPI.
func (m *SessionManager) ListSessionsBySupi(ctx context.Context, supi string) ([]db.PositioningSession, error) {
	sessions, err := m.db.ListPositioningSessions(ctx, supi, -1)
	if err != nil {
		return nil, fmt.Errorf("list sessions for %s: %w", supi, err)
	}

	return sessions, nil
}

// GetActiveSessionBySupi returns the most recent active session for a SUPI, if any.
func (m *SessionManager) GetActiveSessionBySupi(ctx context.Context, supi string) (*db.PositioningSession, error) {
	sessions, err := m.db.ListPositioningSessions(ctx, supi, int(SessionStatusActive))
	if err != nil {
		return nil, fmt.Errorf("get active session for %s: %w", supi, err)
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	return &sessions[0], nil
}
