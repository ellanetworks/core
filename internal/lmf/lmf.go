// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/nrppa"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// LPPHandler is the interface for sending LPP messages to a UE via the AMF.
type LPPHandler interface {
	ForwardLPPToUE(ctx context.Context, supi string, lppData []byte) error
}

// LMF is the Location Management Function. It orchestrates positioning
// procedures and exposes a DetermineLocation method that the REST API
// calls to obtain a subscriber's current location.
type LMF struct {
	amf         *amf.AMF
	sessionMgr  *SessionManager
	lppHandler  LPPHandler
	nrppaClient *nrppa.Client
	lppSessions map[string]*lpp.Session // sessionID -> LPP session
	lppMu       sync.RWMutex
}

// New creates an LMF instance that reads UE location from the given AMF.
func New(amfInstance *amf.AMF, d *db.Database) *LMF {
	logger.LmfLog.Info("LMF initialized")

	return &LMF{
		amf:         amfInstance,
		sessionMgr:  NewSessionManager(d),
		nrppaClient: nrppa.New(amfInstance),
		lppSessions: make(map[string]*lpp.Session),
	}
}

// SetLPPHandler configures the LPP message handler for UE communication.
func (l *LMF) SetLPPHandler(h LPPHandler) {
	l.lppHandler = h
}

// SessionManager returns the session manager for positioning session lifecycle.
func (l *LMF) SessionManager() *SessionManager {
	return l.sessionMgr
}

// ForwardLPPToLMF is a helper that forwards an LPP payload from the AMF to the LMF
// for processing. Called by AMF when an UL NAS Transport carries LPP.
func ForwardLPPToLMF(lmf *LMF, ctx context.Context, supi etsi.SUPI, lppData []byte) error {
	if lmf == nil {
		logger.LmfLog.Debug("LMF is nil, dropping LPP payload")
		return nil
	}

	logger.LmfLog.Debug("LPP payload received from UE",
		zap.String("supi", supi.String()),
		zap.Int("payload_len", len(lppData)),
	)

	msg, err := lpp.ParseLPPMessage(lppData)
	if err != nil {
		logger.LmfLog.Error("failed to parse LPP message", zap.Error(err))
		return fmt.Errorf("parse LPP message: %w", err)
	}

	lmf.lppMu.RLock()

	var activeSession *lpp.Session

	for _, session := range lmf.lppSessions {
		if session.Supi() == supi.String() && session.State() != lpp.SessionFailed {
			activeSession = session
			break
		}
	}

	lmf.lppMu.RUnlock()

	if activeSession == nil {
		logger.LmfLog.Warn("no active LPP session for UE, dropping message",
			zap.String("supi", supi.String()),
		)

		return fmt.Errorf("no active session")
	}

	if err := activeSession.HandleResponse(msg); err != nil {
		logger.LmfLog.Error("failed to handle LPP response", zap.Error(err))
		activeSession.Fail()

		return fmt.Errorf("handle LPP response: %w", err)
	}

	return nil
}

// RegisterLPPSession adds an LPP session to the LMF's session map.
func (l *LMF) RegisterLPPSession(sessionID string, session *lpp.Session) {
	l.lppMu.Lock()
	defer l.lppMu.Unlock()

	l.lppSessions[sessionID] = session
}

// GetLPPSession returns the LPP session for a given session ID.
func (l *LMF) GetLPPSession(sessionID string) *lpp.Session {
	l.lppMu.RLock()
	defer l.lppMu.RUnlock()

	return l.lppSessions[sessionID]
}

// DeregisterLPPSession removes an LPP session from the LMF's session map.
// Call this when a session reaches a terminal state (completed, failed, or cancelled).
func (l *LMF) DeregisterLPPSession(sessionID string) {
	l.lppMu.Lock()
	defer l.lppMu.Unlock()

	delete(l.lppSessions, sessionID)
}
