// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

func HandleHandoverNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.HandoverNotify) {
	targetUe, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	targetUe.TouchLastSeen()

	if msg.UserLocationInformation != nil {
		targetUe.UpdateLocation(ctx, *msg.UserLocationInformation)
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

	var present []amf.RANSession

	for _, sr := range amfUe.SmContextRefs() {
		if sr.Ref == "" {
			continue
		}

		if _, ok := admitted[sr.PduSessionID]; ok {
			present = append(present, amf.RANSession{PduSessionID: sr.PduSessionID})
		}
	}

	amfInstance.ReconcileSessionsToRAN(ctx, amfUe, amf.RANSessions{
		Present:       present,
		Authoritative: true,
	}, func(ctx context.Context, ref string, _ []byte) ([]byte, error) {
		return nil, amfInstance.Session.UpdateSmContextN2HandoverComplete(ctx, ref)
	})

	// Move the UE onto the target and clear the FSM, gated on the UE still being
	// present after the unlocked user-plane switch; only then end the procedure and
	// release the source (TS 23.502).
	if !amfInstance.FinishHandoverCommit(amfUe, targetUe) {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Notify: UE released during the user-plane switch")
		return
	}

	logger.WithTrace(ctx, targetUe.Log).Info("Handle Handover notification Finished")

	sourceUe.ReleaseAction = amf.UeContextReleaseHandover

	sourceUe.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkSuccessfulHandover})
}
