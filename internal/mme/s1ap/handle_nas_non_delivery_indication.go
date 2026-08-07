// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleNASNonDeliveryIndication reports a downlink NAS-PDU the eNB could not deliver
// to the UE (TS 36.413 §8.6). It is report-only: the NAS-PDU IE is the undelivered
// downlink message, so feeding it into the uplink path would fail the integrity
// check, perturb the uplink NAS count, and pre-security could mint a bogus context.
// Retransmission is the NAS layer's.
func handleNASNonDeliveryIndication(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseNASNonDeliveryIndication(value)
	if err != nil {
		// §10.3.5: the procedure has no message to report an unsuccessful
		// outcome, so the receiver "shall terminate the procedure and initiate
		// the Error Indication procedure".
		logger.From(ctx, radio.Log).Warn("failed to decode NAS Non Delivery Indication", zap.Error(err))
		sendParseErrorIndication(m, ctx, radio.Conn, s1ap.ProcNASNonDeliveryIndication, err)

		return
	}

	ue, ueConn, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcNASNonDeliveryIndication, s1ap.TriggeringInitiatingMessage, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID), msg.Diagnostics())

	ue.TouchLastSeen()

	fields := []zap.Field{
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
	}
	if msg.Cause != nil {
		fields = append(fields, zap.String("cause", mme.S1apCauseName(msg.Cause)))
	}

	logger.From(ctx, logger.MmeLog).Debug("NAS Non Delivery Indication", fields...)
}
