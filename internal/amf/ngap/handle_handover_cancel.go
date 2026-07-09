// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverCancel(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverCancel) {
	sourceUe, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, sourceUe.Log).Debug("Handle Handover Cancel", zap.Int64("sourceRanUeNgapID", int64(sourceUe.RanUeNgapID)), zap.Int64("sourceAmfUeNgapID", int64(sourceUe.AmfUeNgapID)))
	sourceUe.TouchLastSeen()

	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem

	if msg.Cause != nil {
		logger.WithTrace(ctx, sourceUe.Log).Debug("Handover Cancel Cause", logger.Cause(causeToString(*msg.Cause)))

		// A malformed cause does not abort the procedure: keep the default and still
		// acknowledge, since HANDOVER CANCEL ACKNOWLEDGE is mandatory (TS 38.413 §8.4.5).
		if p, v, err := getCause(msg.Cause); err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("Get Cause from Handover Cancel Error", zap.Error(err))
		} else {
			causePresent, causeValue = p, v
		}
	}

	amfUe := sourceUe.UeContext()

	// A committing handover (HANDOVER NOTIFY already in flight) is too late to cancel:
	// CancelHandover leaves it for the NOTIFY to finish and reports aborted=false, so
	// the target the UE is moving onto is not released out from under it. Only a
	// cancellable handover ends the procedure and releases a prepared target.
	target, aborted := amfInstance.CancelHandover(amfUe)
	if aborted && target != nil {
		target.ReleaseAction = amf.UeContextReleaseHandover

		if err := target.SendUEContextReleaseCommand(ctx, causePresent, causeValue); err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending UE Context Release Command to target UE", zap.Error(err))
		}
	}

	// HANDOVER CANCEL ACKNOWLEDGE is sent unconditionally — the response to a HANDOVER
	// CANCEL is mandatory and independent of the target-release outcome (TS 38.413 §8.4.5).
	if err := sourceUe.SendHandoverCancelAcknowledge(ctx); err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover cancel acknowledge to source UE", zap.Error(err))
	}
}
