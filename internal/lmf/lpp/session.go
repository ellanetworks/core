// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/lmf/lpp/models"
	lmmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// SessionState tracks the LPP protocol state machine for one positioning session.
type SessionState int

const (
	SessionIdle           SessionState = iota // Initial state, no LPP exchange
	CapabilitiesRequested                     // LMF sent RequestLocationInformation (capabilities)
	CapabilitiesReceived                      // UE replied with ProvideLocationCapabilities
	LocationRequested                         // LMF sent RequestLocationInformation (actual location)
	LocationReceived                          // UE replied with ProvideLocationInformation
	SessionFailed                             // Error occurred, session terminated
)

func (s SessionState) String() string {
	switch s {
	case SessionIdle:
		return "idle"
	case CapabilitiesRequested:
		return "capabilities_requested"
	case CapabilitiesReceived:
		return "capabilities_received"
	case LocationRequested:
		return "location_requested"
	case LocationReceived:
		return "location_received"
	case SessionFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// Session manages the LPP protocol state machine for one UE positioning session.
type Session struct {
	mu             sync.Mutex
	supi           string
	sessionID      string
	method         string
	state          SessionState
	transactionID  byte
	sequenceNumber byte
	lastInbound    []byte
	capabilities   *models.ProvideLocationCapabilities
	locationResult *models.GNSSPositionResult
	log            *zap.Logger
	transferFunc   func(lppMsg []byte) error
	completeFunc   func(result *lmmodels.LocationResult) error
	failFunc       func() error
	cancelFunc     func() error
	deregisterFunc func()
}

// NewSession creates a new LPP session for the given SUPI and positioning method.
func NewSession(supi, sessionID, method string) *Session {
	return &Session{
		supi:          supi,
		sessionID:     sessionID,
		method:        method,
		state:         SessionIdle,
		transactionID: 0x00,
		log:           logger.LmfLog.With(zap.String("supi", supi), zap.String("session_id", sessionID)),
	}
}

// SetTransport sets the functions used to send LPP messages to the UE and
// report session completion/failure back to the session manager.
func (s *Session) SetTransport(transfer func(lppMsg []byte) error, complete func(result *lmmodels.LocationResult) error, fail func() error, cancel func() error, deregister func()) {
	s.transferFunc = transfer
	s.completeFunc = complete
	s.failFunc = fail
	s.cancelFunc = cancel
	s.deregisterFunc = deregister
}

// NextTransactionID returns the next transaction ID and increments the counter.
// Wraps at 255 as per TS 24.030.
func (s *Session) NextTransactionID() byte {
	id := s.transactionID
	s.transactionID = (s.transactionID + 1) & 0xFF

	return id
}

// AckInbound records an inbound PDU for duplicate detection and reserves the
// next downlink sequence number for its acknowledgement. It is safe to call
// from the UL NAS handler goroutine.
//
// duplicate is true when the PDU is byte-identical to the immediately preceding
// one, i.e. a retransmission (TS 37.355 §4.3.4), whose body must not be
// processed again. Sequence-number equality alone is not used: an observed
// handset reuses one uplink sequence number across its distinct capabilities
// and location replies, so that test would drop the second. The acknowledgement
// is owed even for a duplicate (§4.3.3), so ackSeq is always valid.
func (s *Session) AckInbound(data []byte) (ackSeq byte, duplicate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	duplicate = s.lastInbound != nil && bytes.Equal(s.lastInbound, data)
	if !duplicate {
		s.lastInbound = append(s.lastInbound[:0], data...)
	}

	ackSeq = s.sequenceNumber
	s.sequenceNumber++

	return ackSeq, duplicate
}

// NextSequenceNumber returns the next downlink sequence number for this
// session. TS 37.355 §4.3.2 requires one on every message and requires it to
// differ from the last sent in the same direction: a repeat is discarded by the
// peer as a duplicate. It wraps at 255, which the peer accepts because only
// equality with the immediately preceding number matters.
func (s *Session) NextSequenceNumber() byte {
	n := s.sequenceNumber
	s.sequenceNumber++

	return n
}

// StartSession initializes the session and begins the capability exchange.
// For AGNSS-assisted positioning, this sends a RequestLocationInformation
// asking the UE to report its capabilities.
func (s *Session) StartSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != SessionIdle {
		return fmt.Errorf("session already started (state=%s)", s.state)
	}

	s.state = CapabilitiesRequested

	msg, err := EncodeRequestCapabilities(s.NextTransactionID(), s.NextSequenceNumber())
	if err != nil {
		s.state = SessionFailed
		return fmt.Errorf("build request capabilities: %w", err)
	}

	if err := s.send(msg); err != nil {
		s.state = SessionFailed
		return fmt.Errorf("send capabilities request: %w", err)
	}

	return nil
}

// HandleResponse processes a UE response and advances the state machine.
func (s *Session) HandleResponse(msg any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch response := msg.(type) {
	case *models.ProvideLocationCapabilities:
		return s.handleCapabilities(response)
	case *models.ProvideLocationInformation:
		return s.handleLocation(response)
	case *models.Abort:
		s.handleAbort(response)

		return nil
	default:
		return fmt.Errorf("unexpected message type for state %s: %T", s.state, msg)
	}
}

// handleCapabilities processes ProvideLocationCapabilities from the UE.
func (s *Session) handleCapabilities(capMsg *models.ProvideLocationCapabilities) error {
	if s.state != CapabilitiesRequested {
		return fmt.Errorf("unexpected ProvideLocationCapabilities in state %s", s.state)
	}

	s.capabilities = capMsg

	gnss := make([]string, 0, len(capMsg.GNSSCapability.Supported()))
	for _, id := range capMsg.GNSSCapability.Supported() {
		gnss = append(gnss, id.String())
	}

	s.log.Info("received UE capabilities", zap.Strings("gnss", gnss))

	// TS 37.355 §5.2.1: A-GNSS positioning needs the target to support at least
	// one GNSS. A UE that reports none cannot produce a fix, so asking for one
	// only draws an error (notAllRequestedMeasurementsPossible) 30 s later.
	if len(gnss) == 0 {
		s.log.Warn("UE reports no A-GNSS capability", zap.String("state", s.state.String()))
		s.failLocked()

		return nil
	}

	s.state = LocationRequested

	locMsg, err := EncodeRequestLocationInformation(s.NextTransactionID(), s.NextSequenceNumber())
	if err != nil {
		s.state = SessionFailed
		return fmt.Errorf("build request location: %w", err)
	}

	s.log.Info("sending RequestLocationInformation (location)")

	if err := s.send(locMsg); err != nil {
		s.state = SessionFailed
		return fmt.Errorf("send location request: %w", err)
	}

	return nil
}

// handleAbort processes an Abort from the UE. The peer has abandoned the
// procedure and will send nothing further, so the session fails now with the
// cause it gave (TS 37.355 §5.5.3) rather than waiting out the timeout.
func (s *Session) handleAbort(msg *models.Abort) {
	s.log.Warn(
		"UE aborted the LPP procedure",
		zap.String("state", s.state.String()),
		zap.String("abort_cause", msg.Cause),
		zap.Uint8("transaction_id", msg.TransactionID),
	)

	s.failLocked()
}

// handleLocation processes ProvideLocationInformation from the UE.
func (s *Session) handleLocation(msg *models.ProvideLocationInformation) error {
	if s.state != LocationRequested {
		return fmt.Errorf("unexpected ProvideLocationInformation in state %s", s.state)
	}

	// A target that could not compute a position answers with a cause and no
	// locationEstimate (TS 37.355 §6.5.2). The zero value is the absence of a
	// fix, so reporting it would place the subscriber at (0, 0).
	if !msg.HasEstimate {
		cause := msg.FailureCause
		if cause == "" {
			cause = "no locationEstimate and no cause"
		}

		s.log.Warn(
			"UE reported no location estimate",
			zap.String("state", s.state.String()),
			zap.String("failure_cause", cause),
		)

		s.failLocked()

		return nil
	}

	s.locationResult = &msg.GNSSPositionResult
	s.state = LocationReceived

	s.log.Info(
		"received location fix",
		zap.Int32("lat", msg.GNSSPositionResult.Latitude),
		zap.Int32("lon", msg.GNSSPositionResult.Longitude),
		zap.Uint32("h_acc", msg.GNSSPositionResult.HorizontalAccuracy),
	)

	// Signal completion to session manager
	if s.completeFunc != nil {
		result := &lmmodels.LocationResult{
			SUPI:               s.supi,
			Shape:              lmmodels.GADEllipsoidalPoint,
			Latitude:           msg.GNSSPositionResult.Latitude,
			Longitude:          msg.GNSSPositionResult.Longitude,
			Altitude:           msg.GNSSPositionResult.Altitude,
			HorizontalAccuracy: msg.GNSSPositionResult.HorizontalAccuracy,
			VerticalAccuracy:   msg.GNSSPositionResult.VerticalAccuracy,
		}

		if err := s.completeFunc(result); err != nil {
			s.log.Error("failed to complete session", zap.Error(err))
		}
	}

	if s.deregisterFunc != nil {
		s.deregisterFunc()
	}

	return nil
}

// send wraps the transfer function with logging.
func (s *Session) send(lppMsg []byte) error {
	if s.transferFunc == nil {
		return fmt.Errorf("no transfer function set")
	}

	// The LPP PDU is ciphered inside NAS on the wire, so this hex is the only
	// way to inspect what the UE actually receives (decode against TS 37.355).
	s.log.Debug(
		"sending LPP PDU to UE",
		zap.String("state", s.state.String()),
		zap.Int("len", len(lppMsg)),
		zap.String("lpp_hex", hex.EncodeToString(lppMsg)),
	)

	if err := s.transferFunc(lppMsg); err != nil {
		s.log.Error(
			"failed to send LPP PDU to UE",
			zap.String("state", s.state.String()),
			zap.Error(err),
		)

		return err
	}

	return nil
}

// State returns the current session state.
func (s *Session) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state
}

// Supi returns the SUPI for this session.
func (s *Session) Supi() string {
	return s.supi
}

// SessionID returns the positioning session ID.
func (s *Session) SessionID() string {
	return s.sessionID
}

// Method returns the positioning method.
func (s *Session) Method() string {
	return s.method
}

// LocationResult returns the extracted location fix, or nil if not yet received.
func (s *Session) LocationResult() *models.GNSSPositionResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.locationResult
}

// Capabilities returns the UE capabilities, or nil if not yet received.
func (s *Session) Capabilities() *models.ProvideLocationCapabilities {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.capabilities
}

// Fail marks the session as failed.
func (s *Session) Fail() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == SessionFailed || s.state == LocationReceived {
		return
	}

	s.log.Warn("session marked as failed")
	s.failLocked()
}

// failLocked transitions the session to failed and reports it upstream. The
// caller must hold s.mu and should log the specific reason first.
func (s *Session) failLocked() {
	s.state = SessionFailed

	if s.failFunc != nil {
		if err := s.failFunc(); err != nil {
			s.log.Error("failed to report session failure", zap.Error(err))
		}
	}

	if s.deregisterFunc != nil {
		s.deregisterFunc()
	}
}

// Cancel cancels the session.
func (s *Session) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == SessionFailed || s.state == LocationReceived {
		return
	}

	s.state = SessionFailed
	s.log.Info("session cancelled")

	if s.cancelFunc != nil {
		if err := s.cancelFunc(); err != nil {
			s.log.Error("failed to report session cancellation", zap.Error(err))
		}
	}

	if s.deregisterFunc != nil {
		s.deregisterFunc()
	}
}
