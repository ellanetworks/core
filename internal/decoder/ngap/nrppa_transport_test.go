// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"strings"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/nrppa"
)

// buildDownlinkUEAssociatedNRPPaTransportRaw assembles a raw NGAP
// DownlinkUEAssociatedNRPPaTransport carrying the given NRPPa payload.
func buildDownlinkUEAssociatedNRPPaTransportRaw(t *testing.T, amfUeNgapID, ranUeNgapID int64, nrppaPdu []byte) []byte {
	t.Helper()

	raw, err := (&lib.DownlinkUEAssociatedNRPPaTransport{
		AMFUENGAPID: lib.AMFUENGAPID(amfUeNgapID),
		RANUENGAPID: lib.RANUENGAPID(ranUeNgapID),
		RoutingID:   lib.RoutingID{0x00},
		NRPPaPDU:    nrppaPdu,
	}).Marshal()
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

	msg := DecodeNGAPMessage(raw)

	if msg.PDUType != "InitiatingMessage" {
		t.Errorf("PDUType: got %q, want InitiatingMessage", msg.PDUType)
	}

	if msg.ProcedureCode.Value != int64(lib.ProcDownlinkUEAssociatedNRPPaTransport) {
		t.Errorf("ProcedureCode: got %q", msg.ProcedureCode.Label)
	}

	// Summary should mention the decoded NRPPa message kind.
	if want := "NRPPa=E-CID Measurement Initiation Request"; !strings.Contains(msg.Summary, want) {
		t.Errorf("summary %q missing %q", msg.Summary, want)
	}

	var nrppaIE *IE

	for i := range msg.Value.IEs {
		if msg.Value.IEs[i].ID.Value == int64(lib.IDNRPPaPDU) {
			nrppaIE = &msg.Value.IEs[i]
		}
	}

	if nrppaIE == nil {
		t.Fatal("NRPPa-PDU IE not found")
	}

	decoded, ok := nrppaIE.Value.(NRPPaPDU)
	if !ok {
		t.Fatalf("NRPPa-PDU value type: got %T, want NRPPa-PDU", nrppaIE.Value)
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
