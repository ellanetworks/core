// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func HandleHandoverFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.HandoverFailure) {
	if msg.AMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMF UE NGAP ID is nil")
		return
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("Handover Failure Cause", logger.Cause(msg.Cause.String()))
	}

	targetUe := amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*msg.AMFUENGAPID))
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context on this radio", zap.Uint64("amf-ue-id", uint64(*msg.AMFUENGAPID)))
		sendErrorIndication(ctx, ran, msg.AMFUENGAPID, nil, causeUnknownLocalUEID)

		return
	}

	targetUe.TouchLastSeen()

	amfUe := targetUe.UeContext()

	if amfUe == nil || amfInstance.HandoverTarget(amfUe) != targetUe {
		logger.WithTrace(ctx, ran.Log).Warn("ignoring Handover Failure not from the prepared handover target",
			zap.Uint64("amf-ue-id", uint64(*msg.AMFUENGAPID)))

		return
	}

	amfInstance.UnbindHandoverTarget(ctx, amfUe)

	failureCause := causeHOFailureInTarget
	if msg.Cause != nil {
		failureCause = *msg.Cause
	}

	sourceUe := amfInstance.HandoverSource(amfUe)

	switch {
	case amfInstance.HandoverFromEPS(amfUe):
		amfInstance.FailRelocationPreparation(amfUe,
			interworking.TargetRefusal{Cause: amf.S1APHandoverFailureCause(failureCause)})
	case sourceUe == nil:
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
	default:
		amfInstance.ClearHandover(amfUe)

		if sourceUe.Radio() == nil {
			logger.WithTrace(ctx, targetUe.Log).Error("source UE radio is nil, cannot send handover preparation failure")
		} else {
			sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil, msg.TargettoSourceFailureTransparentContainer)
		}
	}

	if err := amfInstance.RemoveUeConn(ctx, targetUe); err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error removing target UE association", zap.Error(err))
	}
}
