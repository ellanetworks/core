// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
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

	// Only the prepared target may fail the handover. A HANDOVER FAILURE arriving on
	// any other association holding this AMF UE NGAP ID must not tear down the
	// in-flight handover (TS 38.413 §8.4.2: the procedure is between the AMF and the
	// target NG-RAN node).
	if amfUe == nil || amfInstance.HandoverTarget(amfUe) != targetUe {
		logger.WithTrace(ctx, ran.Log).Warn("ignoring Handover Failure not from the prepared handover target",
			zap.Uint64("amf-ue-id", uint64(*msg.AMFUENGAPID)))

		return
	}

	// Normally a no-op: a target answering with FAILURE never sent an ACKNOWLEDGE,
	// so nothing bound its endpoint. Not free, though — a target sending both
	// would otherwise leave the downlink aimed at a gNB that admitted nothing.
	unbindHandoverTarget(ctx, amfInstance, amfUe)

	sourceUe := amfInstance.HandoverSource(amfUe)
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
	} else {
		amfInstance.ClearHandover(amfUe)

		// The target's Cause is relayed to the source so it learns why the handover
		// failed. Cause is ignore criticality, so §10.3.5 delivers a HANDOVER FAILURE
		// without it; the Cause in HANDOVER PREPARATION FAILURE is mandatory, so an
		// absent one becomes the generic target failure.
		failureCause := causeHoFailureInTarget
		if msg.Cause != nil {
			failureCause = *msg.Cause
		}

		if sourceUe.Radio() == nil {
			logger.WithTrace(ctx, targetUe.Log).Error("source UE radio is nil, cannot send handover preparation failure")
		} else {
			// §8.4.1.3: where the target supplied a Target to Source Failure
			// Transparent Container, the AMF carries it to the source NG-RAN node,
			// which reads the target's PNI-NPN and protocol support information
			// out of it.
			sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil, msg.TargettoSourceFailureTransparentContainer)
		}
	}

	// HANDOVER FAILURE means the target admitted no resources and holds no UE context
	// (it carries no target RAN UE NGAP ID), so the target association is dropped
	// locally with no UE Context Release Command (TS 38.413 §8.4.2.3).
	if err := amfInstance.RemoveUeConn(ctx, targetUe); err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error removing target UE association", zap.Error(err))
	}
}
