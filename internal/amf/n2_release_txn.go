// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"sync"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type n2Releases struct {
	mu   sync.Mutex
	open map[uint8]*guard.Guard
}

func (ueConn *UeConn) armN2Release(pduSessionID uint8) {
	if ueConn == nil {
		return
	}

	ueConn.n2Releases.mu.Lock()

	if ueConn.n2Releases.open == nil {
		ueConn.n2Releases.open = make(map[uint8]*guard.Guard)
	}

	g, ok := ueConn.n2Releases.open[pduSessionID]
	if !ok {
		g = &guard.Guard{}
		ueConn.n2Releases.open[pduSessionID] = g
	}

	ueConn.n2Releases.mu.Unlock()

	g.ArmOnce(releaseGuardTimeout, func() { ueConn.expireN2Release(pduSessionID, g) })
}

func (ueConn *UeConn) EndN2Release(pduSessionID uint8) {
	if ueConn == nil {
		return
	}

	ueConn.n2Releases.mu.Lock()

	g, ok := ueConn.n2Releases.open[pduSessionID]
	delete(ueConn.n2Releases.open, pduSessionID)

	ueConn.n2Releases.mu.Unlock()

	if ok {
		g.Stop()
	}
}

func (ueConn *UeConn) AbortN2Releases() {
	if ueConn == nil {
		return
	}

	ueConn.n2Releases.mu.Lock()

	open := ueConn.n2Releases.open
	ueConn.n2Releases.open = nil

	ueConn.n2Releases.mu.Unlock()

	for _, g := range open {
		g.Stop()
	}
}

func (ueConn *UeConn) expireN2Release(pduSessionID uint8, want *guard.Guard) {
	ueConn.n2Releases.mu.Lock()

	g, ok := ueConn.n2Releases.open[pduSessionID]
	if ok && g == want {
		delete(ueConn.n2Releases.open, pduSessionID)
	}

	ueConn.n2Releases.mu.Unlock()

	if !ok || g != want {
		return
	}

	ctx := context.Background()

	logger.From(ctx, ueConn.Log()).Warn("no answer to the PDU session resource release; completing it locally",
		zap.Uint8("pdu_session_id", pduSessionID))

	ueConn.SetN2SessionInactive(pduSessionID)

	ue := ueConn.UeContext()
	if ue == nil || ueConn.amf == nil || ueConn.amf.Session == nil {
		return
	}

	smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !ok || smContext == nil {
		return
	}

	removed, err := ueConn.amf.Session.UpdateSmContextN2InfoPduResRelRsp(ctx, smContext.Ref)
	if err != nil {
		logger.From(ctx, ueConn.Log()).Error("could not complete an unanswered PDU session resource release at the SMF",
			zap.Uint8("pdu_session_id", pduSessionID), zap.Error(err))

		return
	}

	if removed {
		ue.DeleteSmContext(pduSessionID)
	}
}
