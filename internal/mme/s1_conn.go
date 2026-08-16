// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"sync/atomic"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// ICSState tracks the S1AP Initial Context Setup progress for one connection
// (TS 36.413 §8.3).
type ICSState int

const (
	// ICSNotStarted: the MME has not sent InitialContextSetupRequest yet.
	ICSNotStarted ICSState = iota
	// ICSPending: InitialContextSetupRequest sent, awaiting response.
	ICSPending
	// ICSCompleted: InitialContextSetupResponse received — radio bearers established.
	ICSCompleted
)

// UeConn is a UE's transient state for one UE-associated logical S1-connection
// (TS 36.413): the S1AP identities, the eNB association, the connection-scoped
// NAS-guard supervision, and any in-flight handover. A fresh one is bound
// on each idle→active transition; the persistent UeContext it belongs to survives
// across them. Fields are guarded by MME.mu unless noted.
type UeConn struct {
	ENBUES1APID               s1ap.ENBUES1APID
	MMEUES1APID               s1ap.MMEUES1APID
	conn                      atomic.Pointer[S1APWriter]
	Log                       *zap.Logger
	ue                        *UeContext
	ServingTAI                s1ap.TAI
	Location                  models.UserLocation
	m                         *MME
	ICS                       ICSState
	secureExchangeEstablished bool
	cipheringStarted          atomic.Bool
	AuthVector                *udm.EPSAV
	resyncTried               bool
	AttachRequestPlain        []byte
	AttachAcceptPlain         []byte
	TauRequestPlain           []byte
	TauAcceptPlain            []byte
	TauReleaseOnComplete      bool
	FiveGSArrival             *FiveGSArrival
	DeferredTAUPlain          []byte
	nasGuard                  guard.Guard
	nasGuardName              string
	esmInfoGuard              guard.Guard
	releaseGuard              guard.Guard
}

type FiveGSArrival struct {
	Sessions *interworking.ArrivingSessions

	RemappedHeldContext bool
}

func (c *UeConn) ArrivedFrom5GS() bool {
	return c != nil && c.FiveGSArrival != nil
}

func (a *FiveGSArrival) ArrivingSessions() *interworking.ArrivingSessions {
	if a == nil {
		return nil
	}

	return a.Sessions
}

// StopReleaseGuard cancels the Release-Complete supervision timer. Nil-safe.
func (c *UeConn) StopReleaseGuard() {
	if c == nil {
		return
	}

	c.releaseGuard.Stop()
}

// Conn returns the UE's current UE-associated S1-connection, or nil when the UE
// is in ECM-IDLE. The atomic load is race-safe against a concurrent connection
// swap under MME.mu.
func (ue *UeContext) Conn() *UeConn {
	if ue == nil {
		return nil
	}

	return ue.active.Load()
}

// UeContext returns the persistent UE context bound to this connection, or nil
// for a bare connection whose first NAS message has not yet warranted one. Read on
// the dispatch goroutine, where the binding set under MME.mu is stable.
func (c *UeConn) UeContext() *UeContext {
	if c == nil {
		return nil
	}

	return c.ue
}

// SecureExchangeEstablished reports whether secure exchange of NAS messages is
// established on the connection (TS 24.301 §4.4.4.3).
func (c *UeConn) SecureExchangeEstablished() bool {
	if c == nil {
		return false
	}

	return c.secureExchangeEstablished
}

// MarkSecureExchangeEstablished records that secure exchange of NAS messages is
// established on the connection (TS 24.301 §4.4.4.3).
func (c *UeConn) MarkSecureExchangeEstablished() {
	if c != nil {
		c.secureExchangeEstablished = true
	}
}

func (c *UeConn) CipheringStarted() bool {
	if c == nil {
		return false
	}

	return c.cipheringStarted.Load()
}

func (c *UeConn) MarkCipheringStarted() {
	if c != nil {
		c.cipheringStarted.Store(true)
	}
}
