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

// HandleUplinkNASTransport routes an uplink NAS message to its UE context
// (TS 38.413 §8.6.3).
func HandleUplinkNASTransport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UplinkNASTransport) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUplinkNASTransport, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		if err := amfInstance.RemoveUeConn(ctx, ueConn); err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("error removing ran ue context", zap.Error(err))
		}

		logger.WithTrace(ctx, ueConn.Log).Error("No UE Context of UeConn",
			zap.Uint64("amf-ue-id", uint64(msg.AMFUENGAPID)),
			zap.Uint32("ran-ue-id", uint32(msg.RANUENGAPID)))

		return
	}

	// Track the UE's current serving cell so a later registration is gated on
	// where the UE now is. An omitted location leaves the last known one
	// standing (§10.3.5).
	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(*msg.UserLocationInformation)
	}

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("NAS handler not set")
		return
	}

	// resolveUE guarantees the UE is connected on this association, so ueConn is
	// the connection the message arrived on.
	amfInstance.NAS.HandleNAS(ctx, ueConn, []byte(msg.NASPDU))
}
