// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverCancel(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverCancel) {
	sourceUe, ok := resolveDecodedUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, sourceUe.Log).Debug("Handle Handover Cancel", zap.Uint32("source-ran-ue-id", uint32(sourceUe.RanUeNgapID)), zap.Uint64("source-amf-ue-id", uint64(sourceUe.AmfUeNgapID)))
	sourceUe.TouchLastSeen()

	cause := ngap.Cause{
		Group: ngap.CauseGroupRadioNetwork,
		Value: int(ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem),
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, sourceUe.Log).Debug("Handover Cancel Cause", logger.Cause(causeToString(*msg.Cause)))

		cause = libCause(msg.Cause)
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

	// The acknowledge is mandatory, so it is sent regardless of the target-release
	// outcome (TS 38.413 §8.4.5).
	sourceUe.SendHandoverCancelAcknowledge(ctx)
}
