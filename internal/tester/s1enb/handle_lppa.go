// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/lppa"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleDownlinkLPPaTransport answers the core's LPPa E-CID Measurement
// Initiation Request with a sample response, so a synchronous
// POST /api/beta/location (E-CID) completes (TS 36.455 §8.2.1). Runs on its own
// goroutine off the receiver so the SCTP send does not block the read loop.
func (e *ENB) handleDownlinkLPPaTransport(value []byte) {
	msg, err := s1ap.ParseDownlinkUEAssociatedLPPaTransport(value)
	if err != nil {
		logger.GnbLogger.Warn("s1enb: undecodable Downlink LPPa Transport", zap.Error(err))
		return
	}

	parsed, err := lppa.ParsePDU([]byte(msg.LPPaPDU))
	if err != nil {
		logger.GnbLogger.Warn("s1enb: undecodable LPPa PDU", zap.Error(err))
		return
	}

	if parsed.Kind != lppa.KindECIDMeasurementInitiationRequest {
		return
	}

	resp, err := e.BuildUplinkLPPaECIDResponse(msg.MMEUES1APID, msg.ENBUES1APID, msg.RoutingID, parsed.Request.ESMLCUEMeasurementID)
	if err != nil {
		logger.GnbLogger.Error("s1enb: build LPPa E-CID response", zap.Error(err))
		return
	}

	if err := e.SendMessage(resp, true); err != nil {
		logger.GnbLogger.Error("s1enb: send LPPa E-CID response", zap.Error(err))
		return
	}

	logger.GnbLogger.Debug("s1enb: sent LPPa E-CID response",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.Int64("esmlc-meas-id", parsed.Request.ESMLCUEMeasurementID))
}
