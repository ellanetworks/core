// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
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

// TestLocationReportMissingMandatoryIE checks that every absent mandatory IE
// is reported with the criticality §9.1.12.3 assigns it: the two UE IDs are
// reject, E-UTRAN CGI, TAI and Request Type are ignore. A decoded message must
// have all of them, so any absence fails the parse.
func TestLocationReportMissingMandatoryIE(t *testing.T) {
	encode := func(t *testing.T, fields []ieField) []byte {
		t.Helper()

		w := per.NewWriter()
		w.WriteBit(false)

		if err := encodeIEContainer(w, per.Aligned, fields); err != nil {
			t.Fatal(err)
		}

		return perBytes(w)
	}

	t.Run("ignore-criticality IEs missing are reported", func(t *testing.T) {
		value := encode(t, []ieField{
			{id: idMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
			{id: idENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(1)},
		})

		var missing *MissingMandatoryIEsError
		if _, err := ParseLocationReport(value); !errors.As(err, &missing) {
			t.Fatalf("error = %v, want *MissingMandatoryIEsError", err)
		}
		// Nothing reject-criticality is absent, so a receiver implementing
		// §10.3.5 answers with an Error Indication rather than a failure.
		if got := missing.RejectedIEs(); len(got) != 0 {
			t.Fatalf("rejected IEs = %v, want none", got)
		}

		if len(missing.IEs) != 3 {
			t.Fatalf("missing IEs = %v, want 3 entries", missing.IEs)
		}
	})

	t.Run("missing reject-criticality IE is listed among all absences", func(t *testing.T) {
		value := encode(t, []ieField{
			{id: idMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		})

		var missing *MissingMandatoryIEsError
		if _, err := ParseLocationReport(value); !errors.As(err, &missing) {
			t.Fatalf("error = %v, want *MissingMandatoryIEsError", err)
		}

		if got := missing.RejectedIEs(); len(got) != 1 || got[0].ID != idENBUES1APID {
			t.Fatalf("rejected IEs = %v, want [eNB-UE-S1AP-ID]", got)
		}
		// The ignore-criticality absences ride along for diagnostics.
		if len(missing.IEs) != 4 {
			t.Fatalf("missing IEs = %v, want 4 entries", missing.IEs)
		}
	})
}
