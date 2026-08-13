// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func movingFrom5GSInIdleMode(req *eps.TrackingAreaUpdateRequest) bool {
	if req == nil || req.OldGUTI.GUTI == nil || req.UEStatus == nil || !req.UEStatus.N1ModeReg {
		return false
	}

	return req.OldGUTIType != nil && *req.OldGUTIType == eps.GUTITypeNative
}

func recoverContextFrom5GS(ctx context.Context, m *mme.MME, conn *mme.UeConn, pdu []byte) (*mme.UeContext, []byte) {
	plain, ok := unprotectedBody(pdu)
	if !ok {
		return nil, nil
	}

	req, err := eps.ParseTrackingAreaUpdateRequest(plain)
	if !decoded(ctx, "TrackingAreaUpdateRequest", err) || !movingFrom5GSInIdleMode(req) {
		return nil, nil
	}

	resp, err := m.FetchEPSContext(ctx, *req.OldGUTI.GUTI, pdu)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Info("the 5GS peer returned no context for an inter-system change; the update is rejected",
			zap.Error(err))

		return nil, nil
	}

	ue := mme.NewUeContext()
	ue.SetSupi(resp.SUPI)

	if err := ue.InstallRelocatedSecurityContext(resp.Security, mme.MintAuthProofForInterworking()); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to install the EPS context mapped from 5GS", zap.Error(err))

		return nil, nil
	}

	ue.Ambr = &models.Ambr{Uplink: resp.AMBRUplink, Downlink: resp.AMBRDownlink}
	ue.TransitionTo(mme.EMMRegistrationInitiated)

	m.AttachUeConn(ue, conn)

	conn.ArrivingFrom5GS = &resp

	logger.From(ctx, logger.MmeLog).Info("recovered the UE's context from 5GS for an idle-mode change",
		zap.String("imsi", ue.IMSI()), zap.Int("pdu-sessions", len(resp.PDNConnections)))

	return ue, plain
}

func unprotectedBody(pdu []byte) ([]byte, bool) {
	sht, err := eps.PeekSecurityHeaderType(pdu)
	if err != nil {
		return nil, false
	}

	switch sht {
	case eps.SHTPlain:
		return pdu, true
	case eps.SHTIntegrityProtected, eps.SHTIntegrityProtectedNewContext:
		if len(pdu) < 6 {
			return nil, false
		}

		return pdu[6:], true
	default:
		return nil, false
	}
}

// answered reports that the update was replied to here, either by the reject an
// empty transfer draws or by the security mode command a re-key defers it behind.
func completeIdleMobilityFrom5GS(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn,
	req *eps.TrackingAreaUpdateRequest, plain []byte,
) (answered bool) {
	arriving := ueConn.ArrivingFrom5GS
	if arriving == nil {
		return false
	}

	ueConn.ArrivingFrom5GS = nil

	// Before the sessions move, so a prior context for the same subscriber gives up
	// its M-TMSI and EPS bearers first and the transfer cannot be undone by
	// superseding it afterwards. The 5GS peer verified the update, which is what
	// entitles this context to the subscriber's identity (TS 24.301 §4.4.4.3).
	m.CommitUEIdentity(ctx, ue, mme.MintAuthProofForInterworking())

	transferred := m.AdoptIdlePDNs(ctx, ue, arriving.PDNConnections)

	// TS 23.401 §5.3.3.1 step 8, which TS 23.502 §4.11.1.3.2 step 7-14 performs:
	// with no bearer context at all the update is rejected. Cause #10 sends the UE
	// to EMM-DEREGISTERED.NORMAL-SERVICE to attach afresh (TS 24.301 §5.5.3.2.5,
	// §5.5.3.3.5). The 5GS peer is left unacknowledged so it keeps the sessions
	// this MME could not take (TS 23.401 §5.3.3.1 step 7).
	if len(transferred) == 0 {
		logger.From(ctx, logger.MmeLog).Info("no PDU session of a UE arriving from 5GS could become a PDN connection; rejecting the update",
			zap.String("imsi", ue.IMSI()), zap.Int("offered", len(arriving.PDNConnections)))

		rejectTrackingAreaUpdate(ctx, m, ue, ueConn, eps.EMMCauseImplicitlyDetached)

		return true
	}

	if err := m.AckEPSContext(ctx, ue.Supi(), transferred); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("the 5GS peer refused the context acknowledgement for an idle-mode change",
			zap.Error(err), zap.String("imsi", ue.IMSI()))
	}

	logger.From(ctx, logger.MmeLog).Info("adopted the PDU sessions of a UE arriving from 5GS in idle mode",
		zap.String("imsi", ue.IMSI()), zap.Int("adopted", len(transferred)),
		zap.Int("offered", len(arriving.PDNConnections)))

	return changeNASAlgorithmsForMappedContext(ctx, m, ue, ueConn, req, plain, arriving.Security.Algorithms)
}

func changeNASAlgorithmsForMappedContext(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn,
	req *eps.TrackingAreaUpdateRequest, plain []byte, current interworking.EPSNASAlgorithms,
) bool {
	_, changed, err := m.NASAlgorithmsForMappedContext(ctx, ue.UeNetCap(), current)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not compare the mapped context's NAS algorithms with the operator policy",
			zap.Error(err), zap.String("imsi", ue.IMSI()))

		return false
	}

	if !changed {
		return false
	}

	logger.From(ctx, logger.MmeLog).Info("the mapped EPS context uses algorithms this MME does not select; re-keying before the update",
		zap.String("imsi", ue.IMSI()))

	ueConn.DeferredTAU = req
	ueConn.DeferredTAUPlain = plain

	startSecurityMode(ctx, m, ue, ueConn, rekeyedKeys)

	return true
}
