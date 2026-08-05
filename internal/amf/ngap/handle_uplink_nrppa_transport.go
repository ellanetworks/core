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

// HandleUplinkUEAssociatedNRPPaTransport stores a gNB-relayed NRPPa PDU on its UE
// context for the LMF to correlate and decode (TS 38.413 §8.14). The PDU is
// opaque to the NGAP layer; the LMF matches it by the LMF-UE-Measurement-ID inside.
func HandleUplinkUEAssociatedNRPPaTransport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UplinkUEAssociatedNRPPaTransport) {
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, &msg.AMFUENGAPID, &msg.RANUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()

	ue := ueConn.UeContext()
	if ue == nil {
		logger.From(ctx, ran.Log).Warn("no AMF UE context for NRPPa transport",
			zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)))

		return
	}

	ue.SetNRPPaMessage(msg.NRPPaPDU)

	logger.From(ctx, ran.Log).Debug("stored uplink NRPPa PDU",
		zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)),
		zap.Int("payload-len", len(msg.NRPPaPDU)),
	)
}
