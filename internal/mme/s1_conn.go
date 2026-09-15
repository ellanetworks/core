// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync/atomic"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
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

const enbUES1APIDUnspecified s1ap.ENBUES1APID = 0xFFFFFFFF

// UeConn is a UE's transient state for one UE-associated logical S1-connection
// (TS 36.413): the S1AP identities, the eNB association, the connection-scoped
// NAS-guard supervision, and any in-flight handover. A fresh one is bound
// on each idle→active transition; the persistent UeContext it belongs to survives
// across them. Fields are guarded by MME.mu unless noted.
type UeConn struct {
	enbUES1APID               atomic.Uint32
	MMEUES1APID               s1ap.MMEUES1APID
	conn                      atomic.Pointer[S1APWriter]
	logFields                 atomic.Pointer[[]zap.Field]
	baseLogFields             atomic.Pointer[[]zap.Field]
	supi                      atomic.Pointer[string]
	ue                        *UeContext
	ServingTAI                s1ap.TAI
	Location                  models.UserLocation
	m                         *MME
	ics                       atomic.Int32
	secureExchangeEstablished bool
	cipheringStarted          atomic.Bool
	AuthVector                *udm.EPSAV
	resyncTried               bool
	AttachRequestPlain        []byte
	AttachAcceptPlain         []byte
	TauRequestPlain           []byte
	TauAcceptPlain            []byte
	TauReleaseOnComplete      bool
	TauRejectCause            *eps.EMMCause
	FiveGSArrival             *FiveGSArrival
	DeferredTAUPlain          []byte
	nasGuard                  guard.Guard
	deferredCause             atomic.Pointer[s1ap.Cause]
	deferGuard                guard.Guard
	nasGuardName              string
	esmInfoGuard              guard.Guard
	releaseGuard              guard.Guard
}

type FiveGSArrival struct {
	Sessions *interworking.ArrivingSessions

	RemappedHeldContext bool
}

func (c *UeConn) ENBUES1APID() s1ap.ENBUES1APID {
	return s1ap.ENBUES1APID(c.enbUES1APID.Load())
}

func (c *UeConn) setENBUES1APID(enbUEID s1ap.ENBUES1APID) {
	c.enbUES1APID.Store(uint32(enbUEID))
}

func (c *UeConn) LogFields() []zap.Field {
	if c == nil {
		return nil
	}

	if f := c.logFields.Load(); f != nil {
		return *f
	}

	return nil
}

func (c *UeConn) Log(ctx context.Context) *zap.Logger {
	return logger.From(ctx, logger.MmeLog, c.LogFields()...)
}

func (c *UeConn) setLogFields(fields []zap.Field) {
	c.logFields.Store(&fields)
}

func (c *UeConn) bindLogFields(base []zap.Field) {
	c.baseLogFields.Store(&base)
	c.refreshLog()
}

func (c *UeConn) bindSupi(supi etsi.SUPI) {
	if c == nil || !supi.IsValid() {
		return
	}

	s := supi.String()
	c.supi.Store(&s)
	c.refreshLog()
}

func (c *UeConn) refreshLog() {
	base := c.baseLogFields.Load()
	if base == nil {
		return
	}

	fields := make([]zap.Field, 0, len(*base)+3)
	fields = append(fields, *base...)

	if supi := c.supi.Load(); supi != nil {
		fields = append(fields, logger.SUPI(*supi))
	}

	fields = append(fields, logger.MMEUeS1apID(uint32(c.MMEUES1APID)))

	if enbUEID := c.ENBUES1APID(); enbUEID != enbUES1APIDUnspecified {
		fields = append(fields, logger.ENBUeS1apID(uint32(enbUEID)))
	}

	c.setLogFields(fields)
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

func (c *UeConn) ICS() ICSState {
	if c == nil {
		return ICSNotStarted
	}

	return ICSState(c.ics.Load())
}

func (c *UeConn) SetICS(state ICSState) {
	if c == nil {
		return
	}

	c.ics.Store(int32(state))
}
