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

	sourceUe := amfUe.HandoverSource()
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
		return
	}

	// Advance the FSM hoPrepared→hoCommitting; an out-of-order Handover Notify (no
	// prepared handover) does not match and is dropped before the user plane is
	// switched. The context is ended below in lockstep with End(N2Handover).
	if !amfUe.MarkHandoverCommitting() {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Notify with no prepared handover; dropping")
		return
	}

	logger.WithTrace(ctx, targetUe.Log).Info("Handle Handover notification Finished")

	if conn := amfUe.NasConn(); conn != nil {
		conn.Procedures.End(procedure.N2Handover)
	}

	amfUe.ClearHandover()

	// Per 3GPP TS 23.502 §4.9.1.3.3 step 7, the SMF sends N4 Session
	// Modification to the UPF with the new AN tunnel info at this point.
	for _, sr := range amfUe.SmContextRefs() {
		if sr.Ref == "" {
			continue
		}

		if err := amfInstance.Smf.UpdateSmContextN2HandoverComplete(ctx, sr.Ref); err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("failed to update SM context for handover completion",
				zap.String("smContextRef", sr.Ref), zap.Error(err))

			continue
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
