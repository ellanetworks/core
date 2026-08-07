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

	// A committing handover (HANDOVER NOTIFY in flight) is too late to cancel:
	// CancelHandover reports aborted=false and leaves the target for the NOTIFY, so it
	// is not released out from under the UE moving onto it (TS 38.413 §8.4.5).
	target, aborted := amfInstance.CancelHandover(amfUe)
	if aborted && target != nil {
		target.ReleaseAction = amf.UeContextReleaseHandover

		target.SendUEContextReleaseCommand(ctx, cause)
	}

	if aborted {
		amfInstance.UnbindHandoverTarget(ctx, amfUe)
	}

	// Sent after the unbind, not before: TS 36.413 §8.4.5.2 has the core release
	// the handover-preparation resources and then acknowledge, so the source is
	// not told the UE is back on it while the downlink still points at the target.
	// A missing acknowledge is not fatal either way — the source treats it as
	// success (TS 38.413 §8.4.5.4).
	sourceUe.SendHandoverCancelAcknowledge(ctx)
}
