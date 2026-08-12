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

	amfUe := targetUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("UeContext is nil")
		return
	}

	fromEPS := amfInstance.HandoverFromEPS(amfUe)

	sourceUe := amfInstance.HandoverSource(amfUe)
	if sourceUe == nil && !fromEPS {
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
		return
	}

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

	if !amfInstance.FinishHandoverCommit(amfUe, targetUe) {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Notify: UE released during the user-plane switch")

		amfInstance.ClearHandover(amfUe)

		return
	}

	if msg.UserLocationInformation != nil {
		targetUe.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	logger.WithTrace(ctx, targetUe.Log).Info("Handle Handover notification Finished")

	if fromEPS {
		amfInstance.CompleteRelocationFromEPS(ctx, amfUe)

		return
	}

	sourceUe.ReleaseAction = amf.UeContextReleaseHandover

	sourceUe.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkSuccessfulHandover})
}
