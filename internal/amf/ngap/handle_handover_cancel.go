// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverCancel(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverCancel) {
	sourceUe, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, sourceUe.Log).Debug("Handle Handover Cancel", zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID))
	sourceUe.TouchLastSeen()

	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem

	var err error

	if msg.Cause != nil {
		logger.WithTrace(ctx, sourceUe.Log).Debug("Handover Cancel Cause", logger.Cause(causeToString(*msg.Cause)))

		causePresent, causeValue, err = getCause(msg.Cause)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("Get Cause from Handover Failure Error", zap.Error(err))
			return
		}
	}

	amfUe := sourceUe.UeContext()

	// A committing handover (HANDOVER NOTIFY already in flight) is too late to cancel:
	// CancelHandover leaves it for the NOTIFY to finish and reports aborted=false, so
	// the target the UE is moving onto is not released out from under it. Only a
	// cancellable handover ends the procedure and releases a prepared target; the
	// acknowledge is always sent (TS 38.413 §8.4.5).
	target, aborted := amfInstance.CancelHandover(amfUe)
	if aborted {
		if conn := amfUe.Conn(); conn != nil {
			conn.Parent().EndKeyChainProc(procedure.N2Handover)
		}

		if target != nil {
			target.ReleaseAction = amf.UeContextReleaseHandover

			if err := target.SendUEContextReleaseCommand(ctx, causePresent, causeValue); err != nil {
				logger.WithTrace(ctx, sourceUe.Log).Error("error sending UE Context Release Command to target UE", zap.Error(err))
				return
			}
		}
	}

	if err := sourceUe.SendHandoverCancelAcknowledge(ctx); err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover cancel acknowledge to source UE", zap.Error(err))
		return
	}
}
