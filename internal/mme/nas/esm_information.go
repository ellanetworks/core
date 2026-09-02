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

func requestESMInformation(ctx context.Context, ue *mme.UeContext, ueConn *mme.UeConn, onAbort func(pti uint8)) bool {
	wait := ue.PendingESMInfo()
	if wait == nil {
		return false
	}

	abort := func() {
		w := ue.TakeESMInfoWait()
		if w == nil {
			return
		}

		ueConn.StopESMInfoGuard()
		onAbort(w.PTI)
	}

	esm, err := (&eps.ESMInformationRequest{PTI: nas.ProcedureTransactionIdentity(wait.PTI)}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build ESM Information Request", zap.Error(err))
		abort()

		return true
	}

	logger.From(ctx, logger.MmeLog).Info("requesting deferred ESM information",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", wait.PTI))

	if err := ueConn.SendProtectedNASTransport(ctx, esm, eps.SHTIntegrityProtectedCiphered); err != nil {
		mme.ReportProtectFailure(ctx, ueConn, "ESM Information Request", err)

		// The connection is gone, so the abort's reject could not be protected.
		if errors.Is(err, nas.ErrCountExhausted) {
			ue.TakeESMInfoWait()

			return true
		}

		abort()

		return true
	}

	ueConn.ArmT3489("ESM Information Request", esm, eps.SHTIntegrityProtectedCiphered, func() {
		logger.MmeLog.Info("ESM information not received", zap.String("imsi", ue.IMSI()))
		abort()
	})

	return true
}

func handleESMInformationResponse(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, req *eps.ESMInformationResponse) nasreply.Disposition {
	if req.EPSBearerIdentity != 0 {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response with an assigned EPS bearer identity, ignoring",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", uint8(req.EPSBearerIdentity)))

		return nasreply.Handled()
	}

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
		egress{conn: ueConn}.SendSMStatusFor(ctx, uint8(eps.ESMCauseInvalidPTIValue), pti, uint8(req.EPSBearerIdentity))

		return nasreply.Handled()
	}

	ueConn.StopESMInfoGuard()

	if req.AccessPointName != nil {
		ue.RequestedAPN = string(*req.AccessPointName)
	}

	if id := pduSessionIDFromPCOs(req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions); id != 0 {
		ue.RequestedPDUSessionID = id
	}

	if opts, ok := protocolOptionsFromPCOs(req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions); ok {
		ue.RequestedProtocolOpts = opts
	}

	logger.From(ctx, logger.MmeLog).Info("received deferred ESM information",
		zap.String("imsi", ue.IMSI()), zap.String("apn", ue.RequestedAPN),
		zap.Uint8("pdu_session_id", ue.RequestedPDUSessionID))

	if wait.Standalone != nil {
		resumePDNConnectivity(ctx, m, ue, ueConn, wait.Standalone)

		return nasreply.Handled()
	}

	activateDefaultBearer(ctx, m, ue, ueConn)

	return nasreply.Handled()
}
