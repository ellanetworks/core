// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// detachTypeReattachNotRequired is the network-originating detach type meaning
// the UE shall not re-attach (TS 24.301) — used when a subscriber is
// removed.
const detachTypeReattachNotRequired uint8 = 2

// DetachSubscriber sends a network-initiated DETACH REQUEST (TS 24.301)
// to the attached UE for imsi, if any, when a subscriber is deleted. The UE
// replies with Detach Accept, on which the S1 context is released and removed.
func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue, ok := m.lookupUeByIMSI(imsi)
	if !ok {
		return
	}

	logger.MmeLog.Info("network-initiated detach (subscriber deleted)",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", imsi))

	ue.emmState.store(EMMDeregistered)
	m.sendDownlinkProtected(ctx, ue, &eps.DetachRequestNetwork{TypeOfDetach: detachTypeReattachNotRequired})
}

// onDetachAccept completes a network-initiated detach: the UE has acknowledged,
// so release and delete its context (the UE is already EMM-DEREGISTERED).
func (m *MME) onDetachAccept(ctx context.Context, ue *UeContext) {
	logger.MmeLog.Info("Detach Accept", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))
	m.releaseUEContext(ctx, ue, causeNASDetach)
}

// isSwitchOffDetach reports whether body is a plain UE-originating DETACH
// REQUEST with the switch-off flag set — the one NAS message the MME accepts
// without integrity protection (TS 24.301).
func isSwitchOffDetach(body []byte) bool {
	if mt, err := eps.PeekMessageType(body); err != nil || mt != eps.MsgDetachRequest {
		return false
	}

	req, err := eps.ParseDetachRequestUE(body)

	return err == nil && req.SwitchOff
}

// onDetachRequest handles a UE-originating DETACH REQUEST (TS 24.301):
// for a non-switch-off detach it replies with Detach Accept, then releases the
// UE's S1 context.
func (m *MME) onDetachRequest(ctx context.Context, ue *UeContext, plain []byte) {
	req, err := eps.ParseDetachRequestUE(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Detach Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Detach Request",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.Bool("switch-off", req.SwitchOff),
		zap.String("imsi", ue.imsi),
	)

	ue.emmState.store(EMMDeregistered)

	if !req.SwitchOff {
		m.sendDownlinkProtected(ctx, ue, &eps.DetachAccept{})
	}

	m.releaseUEContext(ctx, ue, causeNASDetach)
}
