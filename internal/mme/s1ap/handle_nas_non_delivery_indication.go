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
// downlink message, so feeding it back into the uplink path would fail the
// downlink/uplink integrity check, perturb the uplink NAS count, and pre-security
// could mint a bogus context. Any retransmission is the NAS layer's. Mirrors the
// AMF's report-only handler.
func handleNASNonDeliveryIndication(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseNASNonDeliveryIndication(value)
	if err != nil {
		logger.From(ctx, radio.Log).Warn("failed to decode NAS Non Delivery Indication", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	logger.From(ctx, logger.MmeLog).Debug("NAS Non Delivery Indication",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
		zap.String("cause", mme.S1apCauseName(&msg.Cause)))
}
