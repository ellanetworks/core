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
func handleNASNonDeliveryIndication(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseNASNonDeliveryIndication(value)
	if err != nil {
		// §10.3.5: the procedure has no message to report an unsuccessful
		// outcome, so the receiver "shall terminate the procedure and initiate
		// the Error Indication procedure".
		radio.Log(ctx).Warn("failed to decode NAS Non Delivery Indication", zap.Error(err))
		sendParseErrorIndication(ctx, m, radio.Conn, s1ap.ProcNASNonDeliveryIndication, s1ap.TriggeringInitiatingMessage, err)

		return
	}

	_, ueConn, ok := resolveUE(ctx, m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcNASNonDeliveryIndication, s1ap.TriggeringInitiatingMessage, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), msg.Diagnostics())

	fields := []zap.Field{
		logger.MMEUeS1apID(uint32(msg.MMEUES1APID)),
		logger.ENBUeS1apID(uint32(msg.ENBUES1APID)),
	}
	if msg.Cause != nil {
		fields = append(fields, logger.Cause(mme.S1apCauseName(msg.Cause)))
	}

	logger.From(ctx, logger.MmeLog).Debug("NAS Non Delivery Indication", fields...)
}
