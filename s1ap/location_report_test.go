// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestLocationReportRoundTrip(t *testing.T) {
	in := &LocationReport{
		MMEUES1APID: 4242,
		ENBUES1APID: 7,
		EUTRANCGI:   EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1},
		TAI:         TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
		RequestType: RequestType{EventType: EventTypeChangeOfServeCell, ReportArea: ReportAreaECGI},
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(wire)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcLocationReport {
		t.Fatalf("pdu = %T proc = %v, want LocationReport (33)", pdu, im.ProcedureCode)
	}

	out, err := ParseLocationReport(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID {
		t.Fatalf("ids: mme=%d enb=%d", out.MMEUES1APID, out.ENBUES1APID)
	}

	if out.EUTRANCGI != in.EUTRANCGI || out.TAI != in.TAI {
		t.Fatalf("location: cgi=%+v tai=%+v", out.EUTRANCGI, out.TAI)
	}

	if out.RequestType != in.RequestType {
		t.Fatalf("request type = %+v, want %+v", out.RequestType, in.RequestType)
	}
}

func TestLocationReportMissingMandatoryIE(t *testing.T) {
	w := per.NewWriter()

	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		{id: idENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(1)},
	}

	if err := encodeIEContainer(w, per.Aligned, fields); err != nil {
		t.Fatal(err)
	}

	if _, err := ParseLocationReport(perBytes(w)); err == nil {
		t.Fatal("expected missing-mandatory-IE error")
	}
}
