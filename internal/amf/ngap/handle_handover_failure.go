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

func HandleHandoverFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverFailure) {
	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem

	var err error

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("Handover Failure Cause", logger.Cause(causeToString(*msg.Cause)))

		causePresent, causeValue, err = getCause(msg.Cause)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Get Cause from Handover Failure Error", zap.Error(err))
			return
		}
	}

	targetUe := ran.FindUEByAmfUeNgapID(msg.AMFUENGAPID)
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context on this radio", zap.Int64("AmfUeNgapID", msg.AMFUENGAPID))
		sendUnknownLocalUEError(ctx, ran, &msg.AMFUENGAPID, nil)

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
			zap.Int64("AmfUeNgapID", msg.AMFUENGAPID))

		return
	}

	sourceUe := amfInstance.HandoverSource(amfUe)
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
	} else {
		if conn := amfUe.NasConn(); conn != nil {
			conn.Procedures.End(procedure.N2Handover)
		}

		amfInstance.ClearHandover(amfUe)

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}
		if msg.Cause != nil {
			failureCause = *msg.Cause
		}

		if sourceUe.Radio() == nil {
			logger.WithTrace(ctx, targetUe.Log).Error("source UE radio is nil, cannot send handover preparation failure")
		} else {
			err := sourceUe.Radio().NGAPSender.SendHandoverPreparationFailure(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, failureCause, msg.CriticalityDiagnostics)
			if err != nil {
				logger.WithTrace(ctx, targetUe.Log).Error("error sending handover preparation failure", zap.Error(err))
				return
			}
		}
	}

	targetUe.ReleaseAction = amf.UeContextReleaseHandover

	err = targetUe.Radio().NGAPSender.SendUEContextReleaseCommand(ctx, targetUe.AmfUeNgapID, targetUe.RanUeNgapID, causePresent, causeValue)
	if err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error sending UE Context Release Command to target UE", zap.Error(err))
		return
	}
}
