// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
)

// ActiveNasConnection is the state tied to one N1 NAS signalling connection:
// the procedure registry, NAS retransmission timers, and the connection-scoped
// context. Release cancels the context and stops the timers; in-flight handler
// goroutines are not pre-empted, so they must check for nil where they
// dereference the connection.
type ActiveNasConnection struct {
	parent *UeContext

	ctx    context.Context
	cancel context.CancelFunc

	ranUe *RanUe

	Procedures *procedure.Registry

	T3513 guard.Guard
	T3565 guard.Guard
	T3560 guard.Guard
	T3550 guard.Guard
	T3555 guard.Guard
	T3522 guard.Guard

	// secureExchangeEstablished records that secure exchange of NAS messages has
	// been established on this connection (a NAS message has been successfully
	// integrity-checked). Once set, TS 24.501 requires discarding any
	// further message that is not integrity protected or fails the check.
	secureExchangeEstablished bool

	AuthenticationCtx                 *ausf.AuthResult
	AuthFailureCauseSynchFailureTimes int

	RegistrationRequest             *nasMessage.RegistrationRequest
	RegistrationType5GS             uint8
	IdentityTypeUsedForRegistration uint8
	RetransmissionOfInitialNASMsg   bool
	N1N2Message                     *models.N1N2MessageTransferRequest
}

func newActiveNasConnection(parent *UeContext, ranUe *RanUe) *ActiveNasConnection {
	ctx, cancel := context.WithCancel(parent.ctx)

	return &ActiveNasConnection{
		parent:     parent,
		ctx:        ctx,
		cancel:     cancel,
		ranUe:      ranUe,
		Procedures: procedure.NewRegistry(logger.AmfLog),
	}
}

func (conn *ActiveNasConnection) Ctx() context.Context {
	return conn.ctx
}

func (conn *ActiveNasConnection) Parent() *UeContext {
	return conn.parent
}

func (conn *ActiveNasConnection) RanUe() *RanUe {
	return conn.ranUe
}

func (conn *ActiveNasConnection) Release() {
	conn.stopTimers()
	conn.cancel()
	conn.parent.active.CompareAndSwap(conn, nil)
}

// stopTimers stops every retransmission timer owned by this connection.
// Go timers do not observe ctx cancellation; they must be stopped explicitly to
// prevent stale firings on a released RAN UE.
func (conn *ActiveNasConnection) stopTimers() {
	for _, g := range []*guard.Guard{&conn.T3513, &conn.T3565, &conn.T3560, &conn.T3550, &conn.T3555, &conn.T3522} {
		g.Stop()
	}
}
