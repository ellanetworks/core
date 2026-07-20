// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/lppa"
	"github.com/ellanetworks/core/internal/lmf/nrppa"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// LocationSource exposes the per-UE location a control-plane NF holds. The AMF
// (5G) and MME (4G) both satisfy it, so the LMF resolves a subscriber attached on
// either access.
type LocationSource interface {
	IsUERegistered(supi etsi.SUPI) bool
	GetUELocation(supi etsi.SUPI) (models.UserLocation, bool)
}

// LPPHandler is the interface for sending LPP messages to a UE via the AMF.
// correlationID is the LCS correlation identifier the UE uses to route the DL
// message to its LPP session; echoing the UE's identifier keeps a reply on the
// same session (TS 23.273).
type LPPHandler interface {
	ForwardLPPToUE(ctx context.Context, supi string, correlationID, lppData []byte) error
}

// LMF is the Location Management Function. It orchestrates positioning
// procedures and exposes a DetermineLocation method that the REST API
// calls to obtain a subscriber's current location.
type LMF struct {
	amf         *amf.AMF
	mme         *mme.MME
	db          *db.Database
	sessionMgr  *SessionManager
	nrppaClient *nrppa.Client
	lppaClient  *lppa.Client
	lppHandler  LPPHandler
	lppSessions map[string]*lpp.Session // sessionID -> LPP session
	lppMu       sync.RWMutex
	// ackSeq is the fallback downlink sequence counter for acknowledging a stray
	// UE reply that no longer has an active session (TS 37.355 §4.3.2).
	ackSeq atomic.Uint32
}

// New creates an LMF instance that reads UE location from the AMF (5G) and the
// MME (4G).
func New(amfInstance *amf.AMF, mmeInstance *mme.MME, d *db.Database) *LMF {
	logger.LmfLog.Info("LMF initialized")

	return &LMF{
		amf:         amfInstance,
		mme:         mmeInstance,
		db:          d,
		sessionMgr:  NewSessionManager(d),
		nrppaClient: nrppa.New(amfInstance),
		lppaClient:  lppa.New(mmeInstance),
		lppSessions: make(map[string]*lpp.Session),
	}
}

// sources returns the location sources consulted in priority order: the AMF (5G)
// before the MME (4G). A subscriber is attached on at most one access, so the
// first source that owns the UE answers. A nil NF is skipped.
func (l *LMF) sources() []LocationSource {
	srcs := make([]LocationSource, 0, 2)

	if l.amf != nil {
		srcs = append(srcs, l.amf)
	}

	if l.mme != nil {
		srcs = append(srcs, l.mme)
	}

	return srcs
}

// isUERegistered reports whether any access has the UE registered.
func (l *LMF) isUERegistered(supi etsi.SUPI) bool {
	return anyRegistered(l.sources(), supi)
}

// getUELocation returns the location from the access that owns the UE.
func (l *LMF) getUELocation(supi etsi.SUPI) (models.UserLocation, bool) {
	return firstLocation(l.sources(), supi)
}

func anyRegistered(srcs []LocationSource, supi etsi.SUPI) bool {
	for _, s := range srcs {
		if s.IsUERegistered(supi) {
			return true
		}
	}

	return false
}

func firstLocation(srcs []LocationSource, supi etsi.SUPI) (models.UserLocation, bool) {
	for _, s := range srcs {
		if loc, ok := s.GetUELocation(supi); ok {
			return loc, true
		}
	}

	return models.UserLocation{}, false
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
func ForwardLPPToLMF(lmf *LMF, ctx context.Context, supi etsi.SUPI, correlationID, lppData []byte) error {
	if lmf == nil {
		logger.LmfLog.Debug("LMF is nil, dropping LPP payload")
		return nil
	}

	logger.LmfLog.Info("LPP PDU received from UE",
		zap.String("supi", supi.String()),
		zap.Int("len", len(lppData)),
		zap.String("lpp_hex", hex.EncodeToString(lppData)),
	)

	lmf.lppMu.RLock()

	var activeSession *lpp.Session

	for _, session := range lmf.lppSessions {
		if session.Supi() == supi.String() && session.State() != lpp.SessionFailed {
			activeSession = session
			break
		}
	}

	lmf.lppMu.RUnlock()

	// TS 37.355 §4.3.3: acknowledge before touching the body — sending the ack is
	// what stops the UE's retransmission loop (§4.3.4). The ack is owed for any
	// decodable header regardless of whether the body parses.
	duplicate := lmf.acknowledgeLPP(ctx, supi, correlationID, lppData, activeSession)

	if activeSession == nil {
		return fmt.Errorf("no active session")
	}

	// TS 37.355 §4.3.2: a repeated PDU is a retransmission; it is acknowledged
	// (above) but its body must not be processed again.
	if duplicate {
		logger.LmfLog.Debug("dropping duplicate LPP PDU from UE",
			zap.String("supi", supi.String()),
			zap.String("session_id", activeSession.SessionID()),
		)

		return nil
	}

	decoded, err := lpp.DecodeLPPMessage(lppData)
	if err != nil {
		return fmt.Errorf("parse LPP message: %w", err)
	}

	msg, err := decoded.Payload()
	if err != nil {
		return fmt.Errorf("parse LPP message: %w", err)
	}

	if err := activeSession.HandleResponse(msg); err != nil {
		activeSession.Fail()

		return fmt.Errorf("handle LPP response: %w", err)
	}

	return nil
}

// acknowledgeLPP sends an LPP Acknowledgement for an inbound PDU whose header
// requested one (TS 37.355 §4.3.3), and reports whether the PDU is a duplicate.
// A PDU whose header cannot be read is not acknowledged and is treated as
// non-duplicate.
func (lmf *LMF) acknowledgeLPP(ctx context.Context, supi etsi.SUPI, correlationID, lppData []byte, session *lpp.Session) (duplicate bool) {
	info, err := lpp.ParseAckInfo(lppData)
	if err != nil {
		logger.LmfLog.Warn("could not read LPP acknowledgement header",
			zap.String("supi", supi.String()), zap.Error(err))

		return false
	}

	if !info.AckRequested || !info.HasSequence {
		return false
	}

	// The acknowledgement's own downlink sequence number must differ from the
	// previous one (§4.3.2) or the UE discards it as a duplicate. An active session
	// tracks its counter; a stray reply uses the LMF fallback counter.
	var ackSeq byte

	// The ack carries the session's LCS correlation identifier so it routes to the
	// same session (TS 23.273 §6.11.1). A stray reply with no session echoes the
	// identifier the UE sent.
	ackCorrelationID := correlationID

	if session != nil {
		ackSeq, duplicate = session.AckInbound(lppData)

		if id := session.CorrelationID(); len(id) > 0 {
			ackCorrelationID = id
		}
	} else {
		ackSeq = byte(lmf.ackSeq.Add(1))
	}

	ack, err := lpp.BuildAcknowledgement(ackSeq, info.SequenceNumber)
	if err != nil {
		logger.LmfLog.Error("failed to build LPP acknowledgement",
			zap.String("supi", supi.String()), zap.Error(err))

		return duplicate
	}

	if lmf.lppHandler != nil {
		if err := lmf.lppHandler.ForwardLPPToUE(ctx, supi.String(), ackCorrelationID, ack); err != nil {
			logger.LmfLog.Error("failed to send LPP acknowledgement to UE",
				zap.String("supi", supi.String()), zap.Error(err))
		}
	}

	return duplicate
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
