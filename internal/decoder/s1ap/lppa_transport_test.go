// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/lppa"
	"github.com/ellanetworks/core/s1ap"
)

func TestDecodeDownlinkLPPaTransport(t *testing.T) {
	pdu, err := lppa.BuildECIDMeasurementInitiationRequest(11, []lppa.MeasurementQuantityValue{lppa.MeasCellID, lppa.MeasRSRP})
	if err != nil {
		t.Fatal(err)
	}

	wire, err := (&s1ap.DownlinkUEAssociatedLPPaTransport{
		MMEUES1APID: 5,
		ENBUES1APID: 7,
		RoutingID:   3,
		LPPaPDU:     pdu,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(wire)
	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	if msg.ProcedureCode.Label != "DownlinkUEAssociatedLPPaTransport" {
		t.Fatalf("proc = %q", msg.ProcedureCode.Label)
	}

	if v := mustIE(t, msg, idMMEUES1APID).Value; v != uint32(5) {
		t.Fatalf("MME-UE-S1AP-ID = %v", v)
	}

	if v := mustIE(t, msg, idRoutingID).Value; v != uint8(3) {
		t.Fatalf("Routing-ID = %v", v)
	}

	lp := mustIE(t, msg, idLPPaPDU).Value.(LPPaPDU)
	if lp.Protocol != "LPPa" || lp.Decoded == nil {
		t.Fatalf("LPPa-PDU = %+v", lp)
	}

	if lp.Decoded.Kind != "E-CIDMeasurementInitiationRequest" || lp.Decoded.ESMLCUEMeasurementID != 11 {
		t.Fatalf("decoded LPPa = %+v", lp.Decoded)
	}
}

func TestDecodeUplinkLPPaTransport(t *testing.T) {
	ta := int64(42)

	pdu, err := lppa.BuildECIDMeasurementInitiationResponse(11, 9, &lppa.ECIDResult{
		ServingCell:        lppa.ECGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, EUTRACellID: 0x0abcde1},
		ServingCellTAC:     []byte{0x00, 0x09},
		TimingAdvanceType1: &ta,
		RSRP:               []lppa.RSRPItem{{ValueRSRP: 50}},
	})
	if err != nil {
		t.Fatal(err)
	}

	wire, err := (&s1ap.UplinkUEAssociatedLPPaTransport{
		MMEUES1APID: 5,
		ENBUES1APID: 7,
		RoutingID:   3,
		LPPaPDU:     pdu,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(wire)
	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	lp := mustIE(t, msg, idLPPaPDU).Value.(LPPaPDU)
	if lp.Decoded == nil || lp.Decoded.Kind != "E-CIDMeasurementInitiationResponse" {
		t.Fatalf("decoded LPPa = %+v", lp.Decoded)
	}

	if lp.Decoded.ENBUEMeasurementID != 9 || lp.Decoded.Result == nil {
		t.Fatalf("decoded response = %+v", lp.Decoded)
	}

	if lp.Decoded.Result.ServingCellID != "0abcde1" || lp.Decoded.Result.RSRPCells != 1 {
		t.Fatalf("decoded result = %+v", lp.Decoded.Result)
	}

	if lp.Decoded.Result.TimingAdvance == nil || *lp.Decoded.Result.TimingAdvance != 42 {
		t.Fatalf("timing advance = %+v", lp.Decoded.Result.TimingAdvance)
	}
}
