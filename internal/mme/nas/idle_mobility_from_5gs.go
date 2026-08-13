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

	if held, ok := m.LookupUeBySupi(resp.SUPI); ok && held.IdleMobilityFrom5GSPending() {
		return remapHeldContext(ctx, m, held, conn, resp, plain)
	}

	ue := mme.NewUeContext()
	ue.SetSupi(resp.SUPI)

	if err := ue.InstallRelocatedSecurityContext(resp.Security, mme.MintAuthProofForInterworking()); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to install the EPS context mapped from 5GS", zap.Error(err))

		return nil, nil
	}

	ue.Ambr = &models.Ambr{Uplink: resp.AMBRUplink, Downlink: resp.AMBRDownlink}
	ue.TransitionTo(mme.EMMRegistrationInitiated)
	ue.BeginIdleMobilityFrom5GS()

	m.AttachUeConn(ue, conn)

	conn.ArrivingFrom5GS = &interworking.ArrivingSessions{PDN: resp.PDNConnections}

	logger.From(ctx, logger.MmeLog).Info("recovered the UE's context from 5GS for an idle-mode change",
		zap.String("imsi", ue.IMSI()), zap.Int("pdu-sessions", len(resp.PDNConnections)))

	return ue, plain
}

// remapHeldContext resumes an inter-system update the UE repeated before it completed:
// the AMF re-ran the integrity check and derived the mapped context from the new uplink
// NAS COUNT (TS 33.501 §8.6.1), and the PDN connections of the first pass already moved,
// so only the security context is taken over (TS 24.301 §5.5.3.2.7 d).
func remapHeldContext(ctx context.Context, m *mme.MME, held *mme.UeContext, conn *mme.UeConn,
	resp interworking.EPSContextResponse, plain []byte,
) (*mme.UeContext, []byte) {
	if err := held.InstallRelocatedSecurityContext(resp.Security, mme.MintAuthProofForInterworking()); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to install the EPS context remapped for a repeated inter-system change",
			zap.Error(err))

		return nil, nil
	}

	m.AttachUeConn(held, conn)

	conn.RemappedFrom5GS = true

	logger.From(ctx, logger.MmeLog).Info("re-keyed a held context for an inter-system change the UE repeated",
		zap.String("imsi", held.IMSI()))

	return held, plain
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

func completeIdleMobilityFrom5GS(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn,
	plain []byte,
) (answered bool) {
	if ueConn.RemappedFrom5GS {
		ueConn.RemappedFrom5GS = false

		return changeNASAlgorithmsForMappedContext(ctx, m, ue, ueConn, plain,
			interworking.EPSNASAlgorithms{Ciphering: ue.EEA(), Integrity: ue.EIA()})
	}

	arriving := ueConn.ArrivingFrom5GS
	if arriving == nil {
		return false
	}

	ueConn.ArrivingFrom5GS = nil

	m.CommitUEIdentity(ctx, ue, mme.MintAuthProofForInterworking())

	transferred := m.AdoptIdlePDNs(ctx, ue, arriving.PDN)

	if len(transferred) == 0 {
		logger.From(ctx, logger.MmeLog).Info("no PDU session of a UE arriving from 5GS could become a PDN connection; rejecting the update",
			zap.String("imsi", ue.IMSI()), zap.Int("offered", len(arriving.PDN)))

		rejectTrackingAreaUpdate(ctx, m, ue, ueConn, eps.EMMCauseNoEPSBearerContextActivated)

		return true
	}

	if err := m.AckEPSContext(ctx, ue.Supi(), transferred); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("the 5GS peer refused the context acknowledgement for an idle-mode change",
			zap.Error(err), zap.String("imsi", ue.IMSI()))
	}

	logger.From(ctx, logger.MmeLog).Info("adopted the PDU sessions of a UE arriving from 5GS in idle mode",
		zap.String("imsi", ue.IMSI()), zap.Int("adopted", len(transferred)),
		zap.Int("offered", len(arriving.PDN)))

	return changeNASAlgorithmsForMappedContext(ctx, m, ue, ueConn, plain,
		interworking.EPSNASAlgorithms{Ciphering: ue.EEA(), Integrity: ue.EIA()})
}

func changeNASAlgorithmsForMappedContext(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn,
	plain []byte, current interworking.EPSNASAlgorithms,
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

	ueConn.DeferredTAUPlain = plain

	startSecurityMode(ctx, m, ue, ueConn, rekeyedKeys)

	return true
}
