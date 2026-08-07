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

// requestESMInformation asks the UE for the ESM information it deferred and
// reports whether the procedure waits for it (TS 24.301 §6.6.1.2.2). Every abort
// path ends the wait and the T3489 supervision before running onAbort, which
// receives the transaction identity the wait recorded, so nothing the abort needs
// is read from the UE off the NAS goroutine.
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

	esm, err := (&eps.ESMInformationRequest{PTI: nas.ProcedureTransactionIdentity(wait.PTI)}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build ESM Information Request", zap.Error(err))
		abort()

		return true
	}

	naspdu, err := ue.ProtectDownlink(esm, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		mme.ReportProtectFailure(ctx, ue.Conn(), "ESM Information Request", err)

		// An exhausted downlink COUNT has already released the connection, and the
		// abort's reject would fail to protect for the same reason. Clear the wait
		// so no late response resumes a procedure that is over.
		if errors.Is(err, nas.ErrCountExhausted) {
			ue.TakeESMInfoWait()

			return true
		}

		// Returning with no request sent and no T3489 armed would leave the attach
		// waiting on ESM information the UE was never asked for.
		abort()

		return true
	}

	logger.From(ctx, logger.MmeLog).Info("requesting deferred ESM information",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", wait.PTI))

	ue.Conn().ArmT3489("ESM Information Request", naspdu, func() {
		logger.MmeLog.Info("ESM information not received, rejecting",
			zap.String("imsi", ue.IMSI()))
		abort()
	})

	ue.Conn().SendDownlinkNASTransport(ctx, naspdu)

	return true
}

// handleESMInformationResponse takes the APN and PCO the UE deferred and resumes
// the procedure that deferred them — the attach or a standalone PDN CONNECTIVITY
// REQUEST. Its PCO replaces any that request carried (TS 24.301 §6.6.1.2.4).
func handleESMInformationResponse(ctx context.Context, m *mme.MME, ue *mme.UeContext, req *eps.ESMInformationResponse) nasreply.Disposition {
	// TS 24.301 §7.3.2 e): a response naming an assigned or reserved EPS bearer
	// identity is ignored. §6.6.1.2.3 has the UE set "no EPS bearer identity
	// assigned".
	if req.EPSBearerIdentity != 0 {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response with an assigned EPS bearer identity, ignoring",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", uint8(req.EPSBearerIdentity)))

		return nasreply.Handled()
	}

	// TS 24.301 §7.3.1 e): an unassigned or reserved PTI is ignored, and an
	// assigned one matching no ongoing transaction draws ESM STATUS #81.
	pti := uint8(req.PTI)
	if pti == 0 || pti == 255 {
		logger.From(ctx, logger.MmeLog).Warn("ESM Information Response with an unassigned or reserved PTI, ignoring",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti))

		return nasreply.Handled()
	}

	// The transaction is ended only by a response that names it, so one naming
	// another assigned PTI draws #81 and leaves the ongoing procedure running.
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

	// The response's containers replace any the PDN CONNECTIVITY REQUEST carried
	// (TS 24.301 §6.6.1.2.4), and either may hold the identity (§6.5.1.2).
	if req.ProtocolConfigurationOptions != nil || req.ExtendedProtocolConfigurationOptions != nil {
		ue.RequestedPDUSessionID = pduSessionIDFromPCOs(req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions)
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
