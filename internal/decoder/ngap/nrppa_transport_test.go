// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/ellanetworks/core/internal/nrppa"
	freengap "github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

// buildDownlinkUEAssociatedNRPPaTransportRaw assembles a raw NGAP
// DownlinkUEAssociatedNRPPaTransport carrying the given NRPPa payload.
func buildDownlinkUEAssociatedNRPPaTransportRaw(t *testing.T, amfUeNgapID, ranUeNgapID int64, nrppaPdu []byte) []byte {
	t.Helper()

	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	pdu.InitiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport
	pdu.InitiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore
	pdu.InitiatingMessage.Value.Present = ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport
	pdu.InitiatingMessage.Value.DownlinkUEAssociatedNRPPaTransport = new(ngapType.DownlinkUEAssociatedNRPPaTransport)

	ies := &pdu.InitiatingMessage.Value.DownlinkUEAssociatedNRPPaTransport.ProtocolIEs

	amfIE := ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	amfIE.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	amfIE.Criticality.Value = ngapType.CriticalityPresentReject
	amfIE.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentAMFUENGAPID
	amfIE.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: amfUeNgapID}
	ies.List = append(ies.List, amfIE)

	ranIE := ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Criticality.Value = ngapType.CriticalityPresentReject
	ranIE.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: ranUeNgapID}
	ies.List = append(ies.List, ranIE)

	routingIE := ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	routingIE.Id.Value = ngapType.ProtocolIEIDRoutingID
	routingIE.Criticality.Value = ngapType.CriticalityPresentReject
	routingIE.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentRoutingID
	routingIE.Value.RoutingID = &ngapType.RoutingID{Value: []byte{0x00}}
	ies.List = append(ies.List, routingIE)

	nrppaIE := ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	nrppaIE.Id.Value = ngapType.ProtocolIEIDNRPPaPDU
	nrppaIE.Criticality.Value = ngapType.CriticalityPresentReject
	nrppaIE.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentNRPPaPDU
	nrppaIE.Value.NRPPaPDU = &ngapType.NRPPaPDU{Value: nrppaPdu}
	ies.List = append(ies.List, nrppaIE)

	raw, err := freengap.Encoder(pdu)
	if err != nil {
		t.Fatalf("encode NGAP: %v", err)
	}

	return raw
}

func TestDecodeNGAPMessage_DownlinkUEAssociatedNRPPaTransport(t *testing.T) {
	nrppaPdu, err := nrppa.BuildECIDMeasurementInitiationRequest(4, []nrppa.MeasurementQuantityValue{nrppa.MeasSSRSRP})
	if err != nil {
		t.Fatalf("build NRPPa request: %v", err)
	}

	raw := buildDownlinkUEAssociatedNRPPaTransportRaw(t, 5, 4, nrppaPdu)

	msg := ngap.DecodeNGAPMessage(raw)

	if msg.PDUType != "InitiatingMessage" {
		t.Errorf("PDUType: got %q, want InitiatingMessage", msg.PDUType)
	}

	if msg.ProcedureCode.Label != "DownlinkUEAssociatedNRPPaTransport" {
		t.Errorf("ProcedureCode: got %q", msg.ProcedureCode.Label)
	}

	// Summary should mention the decoded NRPPa message kind.
	if want := "NRPPa=E-CID Measurement Initiation Request"; !strings.Contains(msg.Summary, want) {
		t.Errorf("summary %q missing %q", msg.Summary, want)
	}

	var nrppaIE *ngap.IE

	for i := range msg.Value.IEs {
		if msg.Value.IEs[i].ID.Label == "NRPPaPDU" {
			nrppaIE = &msg.Value.IEs[i]
		}
	}

	if nrppaIE == nil {
		t.Fatal("NRPPaPDU IE not found")
	}

	decoded, ok := nrppaIE.Value.(ngap.NRPPaPDU)
	if !ok {
		t.Fatalf("NRPPaPDU value type: got %T, want ngap.NRPPaPDU", nrppaIE.Value)
	}

	if decoded.Protocol != "NRPPa" {
		t.Errorf("protocol: got %q, want NRPPa", decoded.Protocol)
	}

	if decoded.Decoded == nil {
		t.Fatal("decoded message missing")
	}

	if decoded.Decoded.Error != "" {
		t.Fatalf("unexpected decode error: %s", decoded.Decoded.Error)
	}

	if decoded.Decoded.Kind.Label != "E-CID Measurement Initiation Request" {
		t.Errorf("kind: got %q", decoded.Decoded.Kind.Label)
	}

	if decoded.Decoded.Request == nil {
		t.Fatalf("decoded request missing: %+v", decoded.Decoded)
	}

	if decoded.Decoded.Request.LMFUEMeasurementID != 4 {
		t.Errorf("LMF-UE-Measurement-ID: got %d, want 4", decoded.Decoded.Request.LMFUEMeasurementID)
	}

	if len(decoded.Decoded.Request.MeasurementQuantities) != 1 ||
		decoded.Decoded.Request.MeasurementQuantities[0].Label != "ss-RSRP" {
		t.Errorf("measurement quantities: got %+v", decoded.Decoded.Request.MeasurementQuantities)
	}
}
