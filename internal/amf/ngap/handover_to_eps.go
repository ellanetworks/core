// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handoverRequiredToEPS(ctx context.Context, amfInstance *amf.AMF, sourceUe *amf.UeConn, amfUe *amf.UeContext, msg *ngap.HandoverRequired) {
	if msg.TargetID.TargeteNBID == nil {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Target ID is not an eNB]")
		sourceUe.SendHandoverPreparationFailure(ctx, causeUnknownTargetID, nil, nil)

		return
	}

	target, err := amf.ENBIdentityFromNGAP(*msg.TargetID.TargeteNBID)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [unusable eNB identity]", zap.Error(err))
		sourceUe.SendHandoverPreparationFailure(ctx, causeUnknownTargetID, nil, nil)

		return
	}

	requested := make([]uint8, 0, len(msg.PDUSessionResourceListHORqd))

	for _, item := range msg.PDUSessionResourceListHORqd {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, sourceUe.Log).Error("invalid PDU session ID from gNB, leaving it behind", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		requested = append(requested, pduSessionID)
	}

	sourceUe.HandOverType = msg.HandoverType

	prep, err := amfInstance.PrepareHandoverToEPS(amfUe, sourceUe, target, msg.SourceToTargetTransparentContainer, requested)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [handover to EPS could not be prepared]", zap.Error(err))
		sourceUe.SendHandoverPreparationFailure(ctx, causeHOFailureInTarget, nil, nil)

		return
	}

	go completeHandoverToEPS(context.WithoutCancel(ctx), amfInstance, sourceUe, amfUe, prep)
}

func completeHandoverToEPS(ctx context.Context, amfInstance *amf.AMF, sourceUe *amf.UeConn, amfUe *amf.UeContext, prep *amf.RelocationPreparation) {
	resp, err := amfInstance.ForwardRelocation(ctx, prep.Request)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Warn("the EPS peer could not prepare the handover", zap.Error(err))
		amfInstance.AbandonHandoverToEPS(ctx, amfUe)
		sourceUe.SendHandoverPreparationFailure(ctx, handoverToEPSFailureCause(err), nil, nil)

		return
	}

	admitted := make(map[uint8]struct{}, len(resp.AcceptedPDUSessions))
	for _, pduSessionID := range resp.AcceptedPDUSessions {
		admitted[pduSessionID] = struct{}{}
	}

	notHandedOver, ok := amfInstance.MarkHandoverPrepared(amfUe, admitted)
	if !ok {
		logger.WithTrace(ctx, sourceUe.Log).Warn("the handover to EPS was abandoned while the peer prepared it")
		amfInstance.CancelRelocationToEPS(ctx, amfUe)

		return
	}

	container, err := prep.Container.MarshalBinary()
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("failed to encode the N1 mode to S1 mode NAS transparent container", zap.Error(err))
		amfInstance.AbandonHandoverToEPS(ctx, amfUe)
		sourceUe.SendHandoverPreparationFailure(ctx, causeHOFailureInTarget, nil, nil)

		return
	}

	logger.WithTrace(ctx, sourceUe.Log).Info("Handover Command (5GS to EPS)",
		zap.Int("admitted", len(admitted)),
		zap.Int("not-handed-over", len(notHandedOver)))

	sourceUe.SendHandoverCommandToEPS(ctx,
		releaseItems(ctx, sourceUe, notHandedOver, nil),
		ngap.TargetToSourceTransparentContainer(resp.TargetToSource),
		ngap.NASSecurityParametersFromNGRAN(container),
	)

	amfInstance.SuperviseHandoverToEPS(amfUe)
}

func handoverToEPSFailureCause(err error) ngap.Cause {
	switch {
	case errors.Is(err, interworking.ErrUnknownTarget):
		return causeUnknownTargetID
	default:
		return causeHOFailureInTarget
	}
}
