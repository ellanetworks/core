// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// Reports whether the procedure now waits for the response.
func requestESMInformation(ctx context.Context, ue *mme.UeContext, onAbort func(pti uint8)) bool {
	wait := ue.PendingESMInfo()
	if wait == nil {
		return false
	}

	abort := func() {
		w := ue.TakeESMInfoWait()
		if w == nil {
			return
		}

		ue.Conn().StopESMInfoGuard()
		onAbort(w.PTI)
	}

	// TS 24.301 §6.6.1.2.2 leaves the EPS bearer identity unassigned.
	esm, err := (&eps.ESMInformationRequest{PTI: nas.ProcedureTransactionIdentity(wait.PTI)}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build ESM Information Request", zap.Error(err))
		abort()

		return true
	}

	naspdu, err := ue.ProtectDownlink(esm, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		mme.ReportProtectFailure(ctx, ue.Conn(), "ESM Information Request", err)

		// The connection is gone, so the abort's reject could not be protected
		// either; drop the wait without one.
		if errors.Is(err, nas.ErrCountExhausted) {
			ue.TakeESMInfoWait()

			return true
		}

		abort()

		return true
	}

	logger.From(ctx, logger.MmeLog).Info("requesting deferred ESM information",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", wait.PTI))

	ue.Conn().ArmT3489("ESM Information Request", naspdu, func() {
		logger.MmeLog.Info("ESM information not received", zap.String("imsi", ue.IMSI()))
		abort()
	})

	ue.Conn().SendDownlinkNASTransport(ctx, naspdu)

	return true
}

// TS 24.301 §6.6.1.2.4 also has the response's PCO replace any received earlier
// in the attach; the MME reads no uplink PCO, so there is nothing to replace.
func handleESMInformationResponse(ctx context.Context, m *mme.MME, ue *mme.UeContext, req *eps.ESMInformationResponse) nasreply.Disposition {
	// TS 24.301 §7.3.2 e).
	if req.EPSBearerIdentity != 0 {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response with an assigned EPS bearer identity, ignoring",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", uint8(req.EPSBearerIdentity)))

		return nasreply.Handled()
	}

	// TS 24.301 §7.3.1 e).
	pti := uint8(req.PTI)
	if pti == 0 || pti == 255 {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response with an unassigned or reserved PTI, ignoring",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti))

		return nasreply.Handled()
	}

	wait := ue.TakeESMInfoWaitFor(pti)
	if wait == nil {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response for no ongoing transaction",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti))
		egress{conn: ue.Conn()}.SendSMStatusFor(ctx, uint8(eps.ESMCauseInvalidPTIValue), pti, uint8(req.EPSBearerIdentity))

		return nasreply.Handled()
	}

	ue.Conn().StopESMInfoGuard()

	if req.AccessPointName != nil {
		ue.RequestedAPN = string(*req.AccessPointName)
	}

	logger.From(ctx, logger.MmeLog).Info("received deferred ESM information",
		zap.String("imsi", ue.IMSI()), zap.String("apn", ue.RequestedAPN))

	if wait.Standalone != nil {
		resumePDNConnectivity(ctx, m, ue, wait.Standalone)

		return nasreply.Handled()
	}

	activateDefaultBearer(ctx, m, ue)

	return nasreply.Handled()
}
