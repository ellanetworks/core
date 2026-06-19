// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/nrppa"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleDownlinkUEAssociatedNRPPaTransport(gnb *GnodeB, msg *ngapType.DownlinkUEAssociatedNRPPaTransport) error {
	var (
		amfUeNgapID, ranUeNgapID int64
		nrppaPdu                 []byte
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			if ie.Value.AMFUENGAPID != nil {
				amfUeNgapID = ie.Value.AMFUENGAPID.Value
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			if ie.Value.RANUENGAPID != nil {
				ranUeNgapID = ie.Value.RANUENGAPID.Value
			}
		case ngapType.ProtocolIEIDNRPPaPDU:
			if ie.Value.NRPPaPDU != nil {
				nrppaPdu = ie.Value.NRPPaPDU.Value
			}
		}
	}

	if nrppaPdu == nil {
		return fmt.Errorf("NRPPaPDU IE is missing in DownlinkUEAssociatedNRPPaTransport")
	}

	logger.GnbLogger.Debug("Received Downlink UE Associated NRPPa Transport",
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int("NRPPa PDU length", len(nrppaPdu)),
	)

	// Decode the NRPPa PDU and respond to E-CID measurement initiation requests.
	req, err := nrppa.ParseECIDMeasurementInitiationRequest(nrppaPdu)
	if err != nil {
		logger.GnbLogger.Warn("Ignoring non-E-CID-request NRPPa PDU", zap.Error(err))
		return nil
	}

	logger.GnbLogger.Debug("Decoded NRPPa E-CIDMeasurementInitiationRequest",
		zap.Int64("lmfMeasurementID", req.LMFUEMeasurementID),
		zap.Int("reportCharacteristics", req.ReportCharacteristics),
		zap.Int("measurementQuantities", len(req.MeasurementQuantities)),
	)

	return sendNRPPaECIDMeasurementResponse(gnb, amfUeNgapID, ranUeNgapID, req.LMFUEMeasurementID)
}

func sendNRPPaECIDMeasurementResponse(gnb *GnodeB, amfUeNgapID, ranUeNgapID, lmfMeasurementID int64) error {
	opts := &NRPPaECIDResponseOpts{
		AMFUeNgapID:        amfUeNgapID,
		RANUeNgapID:        ranUeNgapID,
		LMFUEMeasurementID: lmfMeasurementID,
		RANUEMeasurementID: 1, // gNB-assigned RAN-UE-Measurement-ID (sample)
		TimingAdvance:      sampleTimingAdvance,
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
