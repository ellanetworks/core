// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleDetachAccept completes a network-initiated detach: the UE has acknowledged,
// so its context is released and deleted (it is already EMM-DEREGISTERED).
func handleDetachAccept(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	m.StopNASGuard(ue)
	logger.MmeLog.Info("Detach Accept", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
	m.ReleaseUEContext(ctx, ue, mme.CauseNASDetach)
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

// handleDetachRequest handles a UE-originating DETACH REQUEST (TS 24.301):
// for a non-switch-off detach it replies with Detach Accept, then releases the
// UE's S1 context.
func handleDetachRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	req, err := eps.ParseDetachRequestUE(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Detach Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Detach Request",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Bool("switch-off", req.SwitchOff),
		zap.String("imsi", ue.IMSI()),
	)

	ue.SetEMMState(mme.EMMDeregistered)

	if !req.SwitchOff {
		m.SendDownlinkProtected(ctx, ue, &eps.DetachAccept{})
	}

	m.ReleaseUEContext(ctx, ue, mme.CauseNASDetach)
}
