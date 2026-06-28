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

func HandleHandoverNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverNotify) {
	targetUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	if msg.UserLocationInformation != nil {
		targetUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}

	amfUe := targetUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("UeContext is nil")
		return
	}

	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
		return
	}

	logger.WithTrace(ctx, targetUe.Log).Info("Handle Handover notification Finished")

	if conn := amfUe.NasConn(); conn != nil {
		conn.Procedures.End(procedure.N2Handover)
	}

	// Per 3GPP TS 23.502 §4.9.1.3.3 step 7, the SMF sends N4 Session
	// Modification to the UPF with the new AN tunnel info at this point.
	fc := amfUe.Current()
	if fc != nil {
		for _, smContext := range fc.SmContextList {
			if smContext.Ref == "" {
				continue
			}

			if err := amfInstance.Smf.UpdateSmContextN2HandoverComplete(ctx, smContext.Ref); err != nil {
				logger.WithTrace(ctx, targetUe.Log).Error("failed to update SM context for handover completion",
					zap.String("smContextRef", smContext.Ref), zap.Error(err))

				continue
			}
		}
	}

	amfUe.AttachRanUe(targetUe)

	sourceUe.ReleaseAction = amf.UeContextReleaseHandover

	err := sourceUe.Radio().NGAPSender.SendUEContextReleaseCommand(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, ngapType.CausePresentRadioNetwork, ngapType.CauseRadioNetworkPresentSuccessfulHandover)
	if err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error sending ue context release command", zap.Error(err))
		return
	}
}
