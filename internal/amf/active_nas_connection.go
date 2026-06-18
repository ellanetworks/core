// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/ausf"
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
	parent *FivegmmContext

	ctx    context.Context
	cancel context.CancelFunc

	ranUe *RanUe

	Procedures *procedure.Registry

	T3513 *Timer
	T3565 *Timer
	T3560 *Timer
	T3550 *Timer
	T3555 *Timer
	T3522 *Timer

	AuthenticationCtx                 *ausf.AuthResult
	AuthFailureCauseSynchFailureTimes int

	RegistrationRequest             *nasMessage.RegistrationRequest
	RegistrationType5GS             uint8
	IdentityTypeUsedForRegistration uint8
	RetransmissionOfInitialNASMsg   bool
	N1N2Message                     *models.N1N2MessageTransferRequest
}

func newActiveNasConnection(parent *FivegmmContext, ranUe *RanUe) *ActiveNasConnection {
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

func (conn *ActiveNasConnection) Parent() *FivegmmContext {
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
// Go time.Timer values do not observe ctx cancellation; they must be
// stopped explicitly to prevent stale firings on a released RAN UE.
func (conn *ActiveNasConnection) stopTimers() {
	for _, t := range []*Timer{conn.T3513, conn.T3565, conn.T3560, conn.T3550, conn.T3555, conn.T3522} {
		if t != nil {
			t.Stop()
		}
	}
}
