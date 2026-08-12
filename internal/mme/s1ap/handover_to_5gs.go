// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func handoverRequiredToFiveGS(m *mme.MME, ctx context.Context, radio *mme.Radio, req *s1ap.HandoverRequired, ue *mme.UeContext, source *mme.UeConn) {
	if req.TargetID.TargetNgRanNodeID == nil {
		logger.From(ctx, logger.MmeLog).Warn("Handover Required to 5GS whose target is not an NG-RAN node",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeUnknownTargetID)

		return
	}

	target, err := mme.NGRANIdentityFromS1AP(*req.TargetID.TargetNgRanNodeID)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("Handover Required to 5GS with an unusable NG-RAN node identity",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.Error(err))
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeUnknownTargetID)

		return
	}

	prep, err := m.PrepareHandoverToFiveGS(ue, source, target, req.SourceToTarget, req.Cause)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Info("handover to 5GS could not be prepared",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.Error(err))
		mme.SendHandoverPreparationFailure(ctx, m, radio.Conn, req.MMEUES1APID, req.ENBUES1APID, causeHOFailureInTarget)

		return
	}

	go completeHandoverToFiveGS(context.WithoutCancel(ctx), m, ue, source, *prep)
}

func completeHandoverToFiveGS(ctx context.Context, m *mme.MME, ue *mme.UeContext, source *mme.UeConn, req interworking.FiveGSRelocationRequest) {
	resp, err := m.RequestRelocationToFiveGS(ctx, req)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("the 5GS peer could not prepare the handover", zap.Error(err))

		if m.AbandonHandoverToFiveGS(ctx, ue, req.ID) {
			mme.SendHandoverPreparationFailure(ctx, m, source.Conn(), source.MMEUES1APID, source.ENBUES1APID,
				handoverToFiveGSFailureCause(err))
		}

		return
	}

	accepted := make(map[uint8]struct{}, len(resp.AcceptedEPSBearers))
	for _, ebi := range resp.AcceptedEPSBearers {
		accepted[ebi] = struct{}{}
	}

	unadmitted, ok := m.MarkRelocationToFiveGSPrepared(ue, accepted)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("the handover to 5GS was abandoned while the peer prepared it")

		if err := m.CancelRelocationToFiveGS(ctx, ue, req.ID); err != nil {
			logger.From(ctx, logger.MmeLog).Info("the 5GS peer had no handover to cancel", zap.Error(err))
		}

		return
	}

	cmd := &s1ap.HandoverCommand{
		MMEUES1APID:    source.MMEUES1APID,
		ENBUES1APID:    source.ENBUES1APID,
		HandoverType:   s1ap.HandoverTypeEPSToFiveGS,
		ERABToRelease:  releaseItems(unadmitted, nil),
		TargetToSource: s1ap.TransparentContainer(resp.TargetToSource),
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal the inter-system Handover Command", zap.Error(err))

		if m.AbandonHandoverToFiveGS(ctx, ue, req.ID) {
			mme.SendHandoverPreparationFailure(ctx, m, source.Conn(), source.MMEUES1APID, source.ENBUES1APID, causeHandoverPrepUnspecific)
		}

		return
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Command (EPS to 5GS)",
		zap.Uint32("mme-ue-id", uint32(source.MMEUES1APID)),
		zap.Int("accepted", len(accepted)),
		zap.Int("released", len(unadmitted)))
	m.SendToRadio(ctx, source.Conn(), mme.S1APProcedureHandoverCommand, b)

	m.SuperviseHandoverToFiveGS(ue, req.ID)
}

func handoverToFiveGSFailureCause(err error) s1ap.Cause {
	switch {
	case errors.Is(err, interworking.ErrUnknownTarget):
		return causeUnknownTargetID
	default:
		return causeHOFailureInTarget
	}
}
