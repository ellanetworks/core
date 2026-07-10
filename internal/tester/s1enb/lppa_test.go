// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"testing"

	"github.com/ellanetworks/core/lppa"
	"github.com/ellanetworks/core/s1ap"
)

func TestBuildUplinkLPPaECIDResponse(t *testing.T) {
	e := &ENB{
		enbID: 0x1a2b3,
		plmn:  s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
		tac:   7,
	}

	wire, err := e.BuildUplinkLPPaECIDResponse(42, 7, 0, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	pdu, err := s1ap.Unmarshal(wire)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcUplinkUEAssociatedLPPaTransport {
		t.Fatalf("pdu = %T proc = %v, want UplinkUEAssociatedLPPaTransport", pdu, im.ProcedureCode)
	}

	msg, err := s1ap.ParseUplinkUEAssociatedLPPaTransport(im.Value)
	if err != nil {
		t.Fatalf("parse transport: %v", err)
	}

	if msg.MMEUES1APID != 42 || msg.ENBUES1APID != 7 {
		t.Fatalf("ids = %d/%d, want 42/7", msg.MMEUES1APID, msg.ENBUES1APID)
	}

	parsed, err := lppa.ParsePDU([]byte(msg.LPPaPDU))
	if err != nil {
		t.Fatalf("parse lppa: %v", err)
	}

	if parsed.Kind != lppa.KindECIDMeasurementInitiationResponse {
		t.Fatalf("kind = %d", parsed.Kind)
	}

	if parsed.Response.ESMLCUEMeasurementID != 5 {
		t.Fatalf("esmlc meas id = %d, want 5", parsed.Response.ESMLCUEMeasurementID)
	}

	res := parsed.Response.Result
	if res == nil || res.APPosition == nil {
		t.Fatalf("result/AP position missing: %+v", res)
	}

	if res.ServingCell.EUTRACellID != uint64(e.eutranCellID()) {
		t.Fatalf("serving cell = %x, want %x", res.ServingCell.EUTRACellID, e.eutranCellID())
	}

	if len(res.RSRP) != 1 || res.RSRP[0].ValueRSRP != sampleLPPaValueRSRP {
		t.Fatalf("rsrp = %+v", res.RSRP)
	}
}
