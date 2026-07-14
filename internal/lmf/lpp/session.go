// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
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

	msg, err := BuildRequestCapabilities(s.NextTransactionID())
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
	s.log.Info("received UE capabilities",
		zap.Strings("gnss", func() []string {
			var ids []string
			for _, id := range capMsg.GNSSCapability.Supported() {
				ids = append(ids, id.String())
			}

			return ids
		}()),
	)

	// For AGNSS-assisted, now request actual location
	s.state = LocationRequested

	locMsg, err := BuildRequestLocationInfo(s.NextTransactionID(), PosMethodGNSS)
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

// handleLocation processes ProvideLocationInformation from the UE.
func (s *Session) handleLocation(msg *models.ProvideLocationInformation) error {
	if s.state != LocationRequested {
		return fmt.Errorf("unexpected ProvideLocationInformation in state %s", s.state)
	}

	s.locationResult = &msg.GNSSPositionResult
	s.state = LocationReceived

	s.log.Info("received location fix",
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

	if err := s.transferFunc(lppMsg); err != nil {
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

	s.state = SessionFailed
	s.log.Warn("session marked as failed")

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
