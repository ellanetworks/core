// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func HandleHandoverCancel(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.HandoverCancel) {
	sourceUe, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, sourceUe.Log).Debug("Handle Handover Cancel", zap.Uint32("source-ran-ue-id", uint32(sourceUe.RanUeNgapID)), zap.Uint64("source-amf-ue-id", uint64(sourceUe.AmfUeNgapID)))
	sourceUe.TouchLastSeen()

	cause := ngap.Cause{
		Group: ngap.CauseGroupRadioNetwork,
		Value: ngap.CauseRadioNetworkHOFailureInTarget,
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, sourceUe.Log).Debug("Handover Cancel Cause", logger.Cause(msg.Cause.String()))

		cause = *msg.Cause
	}

	amfUe := sourceUe.UeContext()

	target, toEPS, aborted := amfInstance.CancelHandover(amfUe)
	if aborted && target != nil {
		target.ReleaseAction = amf.UeContextReleaseHandover

		target.SendUEContextReleaseCommand(ctx, cause)
	}

	if aborted && toEPS {
		amfInstance.CancelRelocationToEPS(ctx, amfUe)
	}

	if aborted {
		amfInstance.UnbindHandoverTarget(ctx, amfUe)
	}

	sourceUe.SendHandoverCancelAcknowledge(ctx)
}
