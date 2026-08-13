// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"slices"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleSecurityModeComplete(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, smc *eps.SecurityModeComplete) nasreply.Disposition {
	if ue.RegStep() != mme.RegStepSecurityMode {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Security Mode Complete outside the security mode sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ueConn.StopNASGuard()

	m.ClearKeyChainBusy(ue)

	var pei etsi.IMEI

	if smc.IMEISV != nil && smc.IMEISV.IMEISV != nil {
		if parsed, err := etsi.NewIMEIFromPEI(smc.IMEISV.IMEISV.String()); err == nil {
			pei = parsed
		} else {
			logger.From(ctx, logger.MmeLog).Warn("failed to parse IMEISV", zap.String("imsi", ue.IMSI()), zap.Error(err))
		}
	}

	ue.MarkSecured(pei)
	ue.PinKeNBFreshness()

	if len(smc.ReplayedNASMessageContainer) > 0 {
		req, err := eps.ParseAttachRequest(smc.ReplayedNASMessageContainer)
		if !decoded(ctx, "AttachRequest", err) {
			logger.From(ctx, logger.MmeLog).Warn("failed to decode replayed NAS message container in Security Mode Complete", zap.Error(err))
			rejectAttach(ctx, m, ue, ueConn, eps.EMMCauseInvalidMandatoryInformation)

			return nasreply.Handled()
		}

		logger.From(ctx, logger.MmeLog).Info("recovered genuine Attach Request from replayed NAS message container", zap.String("imsi", ue.IMSI()))

		ingestAttachRequest(ctx, ue, ueConn, req)
		ueConn.AttachRequestPlain = slices.Clone(smc.ReplayedNASMessageContainer)
	}

	logger.From(ctx, logger.MmeLog).Info("NAS security context established",
		zap.String("imsi", ue.IMSI()),
	)

	if deferred := ueConn.DeferredTAU; deferred != nil {
		plain := ueConn.DeferredTAUPlain
		ueConn.DeferredTAU, ueConn.DeferredTAUPlain = nil, nil

		return handleTrackingAreaUpdate(ctx, m, ue, ueConn, deferred, plain)
	}

	activateDefaultBearer(ctx, m, ue, ueConn)

	return nasreply.Handled()
}
