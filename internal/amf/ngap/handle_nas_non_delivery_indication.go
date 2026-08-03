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

// HandleNASNonDeliveryIndication reports a downlink NAS-PDU the NG-RAN node could
// not deliver (TS 38.413 §8.6.4). Report-only: the NAS-PDU IE is the undelivered
// downlink message, so feeding it back into the uplink path would fail the
// integrity check, perturb the uplink NAS count, and pre-security could mint a
// bogus context. Retransmission is the NAS layer's.
func HandleNASNonDeliveryIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.NASNonDeliveryIndication) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcNASNonDeliveryIndication, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()

	fields := []zap.Field{
		zap.Uint64("amf-ue-id", uint64(msg.AMFUENGAPID)),
		zap.Uint32("ran-ue-id", uint32(msg.RANUENGAPID)),
	}
	if msg.Cause != nil {
		fields = append(fields, logger.Cause(msg.Cause.String()))
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("NAS Non Delivery Indication", fields...)
}
