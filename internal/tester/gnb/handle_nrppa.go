// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/nrppa"
	"go.uber.org/zap"
)

// handleDownlinkUEAssociatedNRPPaTransport answers the core's NRPPa E-CID
// Measurement Initiation Request with a sample response, so a synchronous
// location request completes (TS 38.455 §8.2.1). internal/tester/s1enb answers
// LPPa the same way.
func handleDownlinkUEAssociatedNRPPaTransport(gnb *GnodeB, value []byte) error {
	msg, err := ngap.ParseDownlinkUEAssociatedNRPPaTransport(value)
	if err != nil {
		return fmt.Errorf("undecodable Downlink UE Associated NRPPa Transport: %w", err)
	}

	logger.GnbLogger.Debug("Received Downlink UE Associated NRPPa Transport",
		zap.Uint64("AMF UE NGAP ID", uint64(msg.AMFUENGAPID)),
		zap.Uint32("RAN UE NGAP ID", uint32(msg.RANUENGAPID)),
		zap.Int("NRPPa PDU length", len(msg.NRPPaPDU)),
	)

	parsed, err := nrppa.ParsePDU(msg.NRPPaPDU)
	if err != nil {
		logger.GnbLogger.Warn("Ignoring undecodable NRPPa PDU", zap.Error(err))
		return nil
	}

	if parsed.Kind != nrppa.KindECIDMeasurementInitiationRequest {
		return nil
	}

	req := parsed.Request

	logger.GnbLogger.Debug("Decoded NRPPa E-CIDMeasurementInitiationRequest",
		zap.Int64("lmfMeasurementID", req.LMFUEMeasurementID),
		zap.Int("reportCharacteristics", req.ReportCharacteristics),
		zap.Int("measurementQuantities", len(req.MeasurementQuantities)),
	)

	return sendNRPPaECIDMeasurementResponse(gnb, int64(msg.AMFUENGAPID), int64(msg.RANUENGAPID), msg.RoutingID, req.LMFUEMeasurementID)
}

func sendNRPPaECIDMeasurementResponse(gnb *GnodeB, amfUeNgapID, ranUeNgapID int64, routingID ngap.RoutingID, lmfMeasurementID int64) error {
	opts := &NRPPaECIDResponseOpts{
		AMFUeNgapID:        amfUeNgapID,
		RANUeNgapID:        ranUeNgapID,
		LMFUEMeasurementID: lmfMeasurementID,
		RANUEMeasurementID: 1, // gNB-assigned RAN-UE-Measurement-ID (sample)
		TimingAdvance:      sampleTimingAdvance,
		RoutingID:          routingID,
	}

	pdu, err := BuildNRPPaECIDMeasurementResponse(opts)
	if err != nil {
		return fmt.Errorf("failed to build NRPPa E-CID response: %w", err)
	}

	err = gnb.SendMessage(pdu, NGAPProcedureUplinkNRPPaTransport)
	if err != nil {
		return fmt.Errorf("failed to send NRPPa E-CID response: %w", err)
	}

	logger.GnbLogger.Debug("Sent NRPPa E-CIDMeasurementInitiationResponse",
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int64("lmfMeasurementID", lmfMeasurementID),
	)

	return nil
}
