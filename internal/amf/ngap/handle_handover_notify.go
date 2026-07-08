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
	targetUe, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
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

	sourceUe := amfInstance.HandoverSource(amfUe)
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
		return
	}

	// Advance the FSM hoPrepared→hoCommitting; an out-of-order Handover Notify (no
	// prepared handover) or one from a UeConn that is not the prepared target does not
	// match and is dropped before the user plane is switched or any session released.
	admitted, ok := amfInstance.MarkHandoverCommitting(amfUe, targetUe)
	if !ok {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Notify with no prepared handover for this target; dropping")
		return
	}

	// Complete the sessions the target admitted (the SMF sends N4 Session Modification
	// to the UPF with the new AN tunnel info, TS 23.502), and release those it did not:
	// a session the target rejected cannot continue there, so its core context is freed
	// (TS 23.501 §5.30.3.5 / TS 23.401 §5.5.1.2.2).
	for _, sr := range amfUe.SmContextRefs() {
		if sr.Ref == "" {
			continue
		}

		if _, ok := admitted[sr.PduSessionID]; ok {
			if err := amfInstance.Session.UpdateSmContextN2HandoverComplete(ctx, sr.Ref); err != nil {
				logger.WithTrace(ctx, targetUe.Log).Error("failed to update SM context for handover completion",
					zap.String("smContextRef", sr.Ref), zap.Error(err))
			}

			continue
		}

		if err := amfInstance.Session.ReleaseSmContext(ctx, sr.Ref); err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("failed to release a target-rejected PDU session after handover",
				zap.String("smContextRef", sr.Ref), zap.Uint8("PduSessionID", sr.PduSessionID), zap.Error(err))
		}

		amfUe.DeleteSmContext(sr.PduSessionID)
	}

	// Move the UE onto the target and clear the FSM, gated on the UE still being
	// present after the unlocked user-plane switch; only then end the procedure and
	// release the source (TS 23.502).
	if !amfInstance.FinishHandoverCommit(amfUe, targetUe) {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Notify: UE released during the user-plane switch")
		return
	}

	logger.WithTrace(ctx, targetUe.Log).Info("Handle Handover notification Finished")

	if conn := amfUe.Conn(); conn != nil {
		conn.Parent().EndKeyChainProc(procedure.N2Handover)
	}

	sourceUe.ReleaseAction = amf.UeContextReleaseHandover

	err := sourceUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentRadioNetwork, ngapType.CauseRadioNetworkPresentSuccessfulHandover)
	if err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error sending ue context release command", zap.Error(err))
		return
	}
}
