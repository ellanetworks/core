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

func HandleHandoverCancel(ctx context.Context, ran *amf.Radio, msg decode.HandoverCancel) {
	sourceUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
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

	// Capture the target before ClearHandover wipes it.
	targetUe := amfUe.HandoverTarget()
	if targetUe == nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("N2 Handover between AMF has not been implemented yet")
		return
	}

	if amfUe != nil {
		if conn := amfUe.NasConn(); conn != nil {
			conn.Procedures.End(procedure.N2Handover)
		}

		amfUe.ClearHandover()
	}

	targetUe.ReleaseAction = amf.UeContextReleaseHandover

	err = targetUe.SendUEContextReleaseCommand(ctx, causePresent, causeValue)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending UE Context Release Command to target UE", zap.Error(err))
		return
	}

	err = sourceUe.SendHandoverCancelAcknowledge(ctx)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover cancel acknowledge to source UE", zap.Error(err))
		return
	}
}
