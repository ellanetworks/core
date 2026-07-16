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
type LPPHandler interface {
	// ForwardLPPToUE sends an LPP PDU to the UE. correlationID is the LCS
	// correlation identifier to carry (TS 24.501 §5.4.5.3.2 case c); nil lets the
	// AMF assign a fresh one, which is what a new request does. A reply within an
	// existing session passes the identifier the UE used so it routes to the same
	// session.
	ForwardLPPToUE(ctx context.Context, supi string, correlationID, lppData []byte) error
	// LPPN1ModeSupported reports the UE's LPP-in-N1-mode capability. The second
	// return is false when the capability is unknown (no UE context or no 5GMM
	// capability IE), so "declined" is distinguishable from "unknown".
	LPPN1ModeSupported(supi string) (supported, known bool)
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
	// ackSeq supplies the downlink sequence number for acknowledgements sent
	// outside an active session (a retransmission arriving after teardown), so
	// consecutive acks differ as TS 37.355 §4.3.2 requires.
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
// correlationID is the LCS correlation identifier the UE echoed (may be nil).
func ForwardLPPToLMF(lmf *LMF, ctx context.Context, supi etsi.SUPI, correlationID, lppData []byte) error {
	if lmf == nil {
		logger.LmfLog.Debug("LMF is nil, dropping LPP payload")
		return nil
	}

	// A UE reply is the single most informative event for A-GNSS: log it before
	// parsing so an undecodable PDU is still visible as "the UE did answer".
	logger.LmfLog.Info("LPP PDU received from UE",
		zap.String("supi", supi.String()),
		zap.Int("len", len(lppData)),
		zap.String("lpp_hex", hex.EncodeToString(lppData)),
	)

	lmf.lppMu.RLock()

	var activeSession *lpp.Session

	sessionStates := make([]string, 0, len(lmf.lppSessions))

	for _, session := range lmf.lppSessions {
		sessionStates = append(sessionStates,
			fmt.Sprintf("%s:%s:%s", session.SessionID(), session.Supi(), session.State()))

		if session.Supi() == supi.String() && session.State() != lpp.SessionFailed {
			activeSession = session
			break
		}
	}

	lmf.lppMu.RUnlock()

	// TS 37.355 §4.3.3: acknowledge before touching the body. The ack is owed
	// for any decodable header regardless of whether the body parses, and
	// sending it here is what stops the UE's 2 s retransmission loop (§4.3.4).
	duplicate := lmf.acknowledgeLPP(ctx, supi, correlationID, lppData, activeSession)

	if activeSession == nil {
		logger.LmfLog.Warn("no active LPP session for UE reply; dropping",
			zap.String("supi", supi.String()),
			zap.Strings("registered_sessions", sessionStates),
		)

		return fmt.Errorf("no active session")
	}

	// TS 37.355 §4.3.2: a repeated sequence number is a retransmission; it is
	// acknowledged (done above) but its body must not be processed again.
	if duplicate {
		logger.LmfLog.Debug("dropping duplicate LPP PDU from UE",
			zap.String("supi", supi.String()),
			zap.String("session_id", activeSession.SessionID()),
		)

		return nil
	}

	decoded, err := lpp.DecodeLPPMessage(lppData)
	if err != nil {
		logger.LmfLog.Error("failed to parse LPP PDU from UE",
			zap.String("supi", supi.String()),
			zap.String("lpp_hex", hex.EncodeToString(lppData)),
			zap.Error(err),
		)

		return fmt.Errorf("parse LPP message: %w", err)
	}

	logger.LmfLog.Info("routing LPP PDU to session",
		zap.String("supi", supi.String()),
		zap.String("session_id", activeSession.SessionID()),
		zap.String("state", activeSession.State().String()),
	)

	if err := activeSession.HandleResponse(decoded); err != nil {
		activeSession.Fail()

		return fmt.Errorf("handle LPP response: %w", err)
	}

	return nil
}

// acknowledgeLPP returns an acknowledgement to the UE when the received PDU
// requested one (TS 37.355 §4.3.3), and reports whether the PDU is a duplicate
// of the last one this session received (§4.3.2). A PDU whose header cannot be
// read is not acknowledged and is treated as non-duplicate.
func (lmf *LMF) acknowledgeLPP(ctx context.Context, supi etsi.SUPI, correlationID, lppData []byte, session *lpp.Session) (duplicate bool) {
	info, err := lpp.ParseAckInfo(lppData)
	if err != nil {
		logger.LmfLog.Warn("could not read LPP acknowledgement header",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)

		return false
	}

	if !info.AckRequested || !info.HasSequence {
		return false
	}

	// The acknowledgement's own downlink sequence number must be distinct from
	// the previous one (§4.3.2) or the UE discards it as a duplicate. An active
	// session tracks its counter; a stray reply from a torn-down session uses the
	// LMF's fallback counter so repeated acks still differ.
	var ackSeq byte

	if session != nil {
		ackSeq, duplicate = session.AckInbound(lppData)
	} else {
		ackSeq = byte(lmf.ackSeq.Add(1))
	}

	ack, err := lpp.BuildAcknowledgement(ackSeq, info.SequenceNumber)
	if err != nil {
		logger.LmfLog.Error("failed to build LPP acknowledgement",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)

		return duplicate
	}

	// Reply with the identifier the UE used (TS 23.273 NOTE 11) so the ack routes
	// to the same LPP session on the target; a fresh identifier would not.
	if err := lmf.lppHandler.ForwardLPPToUE(ctx, supi.String(), correlationID, ack); err != nil {
		logger.LmfLog.Error("failed to send LPP acknowledgement to UE",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)
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
