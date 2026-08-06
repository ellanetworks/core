// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// requestESMInformation asks the UE for the ESM information it deferred, and
// reports whether the attach must now wait for it (TS 24.301 §6.6.1.2.2). The
// message carries no EPS bearer identity and replays the PDN CONNECTIVITY
// REQUEST's PTI, and it is sent only once the security context is up — which it
// is, since the default bearer is activated after the security mode procedure.
//
// The guard resends on the first two expiries of T3489 and, on the third, aborts
// the attach with ESM cause #53 (§6.6.1.2.6 a, §6.5.1.6 c).
func requestESMInformation(ctx context.Context, m *mme.MME, ue *mme.UeContext) bool {
	if !ue.AwaitingESMInformation {
		return false
	}

	esm, err := (&eps.ESMInformationRequest{PTI: ue.RequestedPTI}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build ESM Information Request", zap.Error(err))
		rejectAttachESM(ctx, m, ue, eps.ESMCauseESMInformationNotReceived)

		return true
	}

	naspdu, err := ue.ProtectDownlink(esm, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		mme.ReportProtectFailure(ctx, ue.Conn(), "ESM Information Request", err)
		return true
	}

	logger.From(ctx, logger.MmeLog).Info("requesting deferred ESM information",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(ue.RequestedPTI)))

	ue.Conn().ArmT3489("ESM Information Request", naspdu, func() {
		logger.MmeLog.Info("ESM information not received, rejecting attach",
			zap.String("imsi", ue.IMSI()))
		rejectAttachESM(context.Background(), m, ue, eps.ESMCauseESMInformationNotReceived)
	})

	ue.Conn().SendDownlinkNASTransport(ctx, naspdu)

	return true
}

// handleESMInformationResponse takes the APN and PCO the UE deferred and resumes
// the attach (TS 24.301 §6.6.1.2.4). The response's protocol configuration
// options replace any the PDN CONNECTIVITY REQUEST carried, so the PDU session
// identity that decides whether the connection can move to 5GS is read from
// here (§6.5.1.2).
func handleESMInformationResponse(ctx context.Context, m *mme.MME, ue *mme.UeContext, req *eps.ESMInformationResponse) nasreply.Disposition {
	if !ue.AwaitingESMInformation {
		logger.From(ctx, logger.MmeLog).Warn("unexpected ESM Information Response", zap.String("imsi", ue.IMSI()))
		return nasreply.Handled()
	}

	ue.Conn().StopNASGuard()
	ue.AwaitingESMInformation = false

	if req.AccessPointName != nil {
		ue.RequestedAPN = string(*req.AccessPointName)
	}

	if req.ProtocolConfigurationOptions != nil {
		ue.RequestedPDUSessionID = 0
		if id, ok := req.ProtocolConfigurationOptions.PDUSessionID(); ok {
			ue.RequestedPDUSessionID = id
		}
	}

	logger.From(ctx, logger.MmeLog).Info("received deferred ESM information",
		zap.String("imsi", ue.IMSI()), zap.String("apn", ue.RequestedAPN))

	activateDefaultBearer(ctx, m, ue)

	return nasreply.Handled()
}
