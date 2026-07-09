// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleHandoverRequired starts S1 handover preparation toward the target eNB,
// or replies HANDOVER PREPARATION FAILURE (TS 36.413 §8.4.1).
func handleHandoverRequired(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	req, err := s1ap.ParseHandoverRequired(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcHandoverPreparation, err)
		return
	}

	ue, ok := resolveUE(m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	if req.HandoverType != s1ap.HandoverTypeIntraLTE {
		logger.From(ctx, logger.MmeLog).Warn("Handover Required for an unsupported handover type",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.Uint8("handover-type", uint8(req.HandoverType)))
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	if !ue.Secured() || !ue.HasKASME() {
		logger.From(ctx, logger.MmeLog).Warn("Handover Required for a UE without a security context",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverNoSecurity)

		return
	}

	target, ok := m.FindRadioByGlobalENBID(req.TargetID.TargeteNBID.GlobalENBID)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Handover Required for an unknown target eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.String("target-enb", mme.ENBID(req.TargetID.TargeteNBID.GlobalENBID)))
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeUnknownTargetID)

		return
	}

	if target.Conn == radio.Conn {
		logger.From(ctx, logger.MmeLog).Warn("Handover Required targets the source eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	bearers, ok := mme.HandoverBearers(ue)
	if !ok {
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)
		return
	}

	targetMMEID, newNH, newNCC, ok := m.PrepareHandover(ue, target.Conn, req.MMEUES1APID)
	if !ok {
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)
		return
	}

	hoReq := &s1ap.HandoverRequest{
		MMEUES1APID:            targetMMEID,
		HandoverType:           s1ap.HandoverTypeIntraLTE,
		Cause:                  req.Cause,
		UEAMBR:                 handoverUEAMBR(ue),
		ERABToBeSetup:          bearers,
		SourceToTarget:         req.SourceToTarget,
		UESecurityCapabilities: handoverSecurityCapabilities(ue),
		SecurityContext:        s1ap.SecurityContext{NextHopChainingCount: newNCC, NextHopParameter: s1ap.SecurityKey(newNH)},
	}

	b, err := hoReq.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Handover Request", zap.Error(err))
		m.ClearHandover(ue)
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Request",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.String("target-enb", mme.ENBID(req.TargetID.TargeteNBID.GlobalENBID)),
		zap.Int("e-rabs", len(bearers)))
	m.SendS1APConn(ctx, target.Conn, mme.S1APProcedureHandoverRequest, b)

	// Arm the guard only now the HANDOVER REQUEST is sent, so the timer can never race
	// the outbound request (TS 36.413 §8.4).
	m.SuperviseHandover(ue)
}

func handoverUEAMBR(ue *mme.UeContext) s1ap.UEAggregateMaximumBitRate {
	ambrUL, ambrDL := ue.AmbrStrings()

	return s1ap.UEAggregateMaximumBitRate{
		DL: s1ap.BitRate(mme.BitRateToBps(ambrDL)),
		UL: s1ap.BitRate(mme.BitRateToBps(ambrUL)),
	}
}

func handoverSecurityCapabilities(ue *mme.UeContext) s1ap.UESecurityCapabilities {
	uecap, err := eps.ParseUENetworkCapability(ue.UeNetCap())
	if err != nil {
		return s1ap.UESecurityCapabilities{}
	}

	return mme.S1apSecurityCapabilities(uecap)
}
