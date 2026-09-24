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

func requestESMInformation(ctx context.Context, ueConn *mme.UeConn, onAbort func(context.Context, uint8)) bool {
	wait := ueConn.PendingESMInfo()
	if wait == nil {
		return false
	}

	abort := func(ctx context.Context) {
		w := ueConn.TakeESMInfoWait()
		if w == nil {
			return
		}

		ueConn.StopESMInfoGuard()
		onAbort(ctx, w.PTI)
	}

	esm, err := (&eps.ESMInformationRequest{PTI: nas.ProcedureTransactionIdentity(wait.PTI)}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build ESM Information Request", zap.Error(err))
		abort(ctx)

		return true
	}

	logger.From(ctx, logger.MmeLog).Info("requesting deferred ESM information", zap.Uint8("pti", wait.PTI))

	if err := ueConn.SendProtectedNASTransport(ctx, esm, eps.SHTIntegrityProtectedCiphered); err != nil {
		mme.ReportProtectFailure(ctx, ueConn, "ESM Information Request", err)

		// The connection is gone, so the abort's reject could not be protected.
		if errors.Is(err, nas.ErrCountExhausted) {
			ueConn.TakeESMInfoWait()

			return true
		}

		abort(ctx)

		return true
	}

	ueConn.ArmT3489(ctx, "ESM Information Request", esm, eps.SHTIntegrityProtectedCiphered, func(ctx context.Context) {
		logger.From(ctx, logger.MmeLog).Info("ESM information not received")
		abort(ctx)
	})

	return true
}

func handleESMInformationResponse(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, req *eps.ESMInformationResponse) nasreply.Disposition {
	if req.EPSBearerIdentity != 0 {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response with an assigned EPS bearer identity, ignoring", zap.Uint8("ebi", uint8(req.EPSBearerIdentity)))

		return nasreply.Handled()
	}

	pti := uint8(req.PTI)
	if pti == 0 || pti == 255 {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response with an unassigned or reserved PTI, ignoring", zap.Uint8("pti", pti))

		return nasreply.Handled()
	}

	wait := ueConn.TakeESMInfoWaitFor(pti)
	if wait == nil {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response for no ongoing transaction", zap.Uint8("pti", pti))
		egress{conn: ueConn}.SendSMStatusFor(ctx, uint8(eps.ESMCauseInvalidPTIValue), pti, uint8(req.EPSBearerIdentity))

		return nasreply.Handled()
	}

	ueConn.StopESMInfoGuard()

	if req.AccessPointName != nil {
		ueConn.ESMRequest.APN = string(*req.AccessPointName)
	}

	if id := pduSessionIDFromPCOs(req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions); id != 0 {
		ueConn.ESMRequest.PDUSessionID = id
	}

	if opts, ok := protocolOptionsFromPCOs(req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions); ok {
		ueConn.ESMRequest.ProtocolOpts = opts
	}

	logger.From(ctx, logger.MmeLog).Info("received deferred ESM information", zap.String("apn", ueConn.ESMRequest.APN),
		logger.PDUSessionID(ueConn.ESMRequest.PDUSessionID))

	if wait.Standalone != nil {
		resumePDNConnectivity(ctx, m, ue, ueConn, wait.Standalone)

		return nasreply.Handled()
	}

	activateDefaultBearer(ctx, m, ue, ueConn)

	return nasreply.Handled()
}
