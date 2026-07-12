// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// HandleUplinkUEAssociatedNRPPaTransport stores a gNB-relayed NRPPa PDU on its UE
// context for the LMF to correlate and decode (TS 38.413 §8.14). The PDU is
// opaque to the NGAP layer; the LMF matches it by the LMF-UE-Measurement-ID inside.
func HandleUplinkUEAssociatedNRPPaTransport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, m *ngapType.UplinkUEAssociatedNRPPaTransport) {
	if m == nil {
		return
	}

	var (
		amfUeNgapID, ranUeNgapID *int64
		nrppaPDU                 []byte
	)

	for _, ie := range m.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			if ie.Value.AMFUENGAPID != nil {
				amfUeNgapID = &ie.Value.AMFUENGAPID.Value
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			if ie.Value.RANUENGAPID != nil {
				ranUeNgapID = &ie.Value.RANUENGAPID.Value
			}
		case ngapType.ProtocolIEIDNRPPaPDU:
			if ie.Value.NRPPaPDU != nil {
				nrppaPDU = ie.Value.NRPPaPDU.Value
			}
		}
	}

	if nrppaPDU == nil {
		logger.From(ctx, ran.Log).Warn("Uplink NRPPa transport received but NRPPaPDU IE is missing")
		return
	}

	ueConn, ok := resolveUE(ctx, amfInstance, ran, ranUeNgapID, amfUeNgapID)
	if !ok {
		return
	}

	ue := ueConn.UeContext()
	if ue == nil {
		logger.From(ctx, ran.Log).Warn("no AMF UE context for NRPPa transport",
			zap.Int64("amf-ue-id", int64(ueConn.AmfUeNgapID)))

		return
	}

	ue.SetNRPPaMessage(nrppaPDU)

	logger.From(ctx, ran.Log).Debug("stored uplink NRPPa PDU",
		zap.Int64("amf-ue-id", int64(ueConn.AmfUeNgapID)),
		zap.Int("payload-len", len(nrppaPDU)),
	)
}
