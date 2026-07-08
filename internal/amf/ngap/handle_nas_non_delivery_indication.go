// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleNasNonDeliveryIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.NASNonDeliveryIndication) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	// TS 38.413 §8.6.4: the NAS-PDU IE carries the *downlink* NAS message the RAN could
	// not deliver (typically because the UE moved). It is report-only — feeding it back
	// into the uplink path would fail the downlink/uplink integrity check, perturb the
	// uplink NAS count, and pre-security could mint a bogus context. Any retransmission
	// is the NAS layer's to decide, not an echo of the reported PDU.
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle NAS Non Delivery Indication", zap.Int64("RanUeNgapID", ueConn.RanUeNgapID), zap.Int64("AmfUeNgapID", ueConn.AmfUeNgapID), logger.Cause(causeToString(msg.Cause)))
	ueConn.TouchLastSeen()
}
