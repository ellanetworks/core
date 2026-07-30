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
		EUTRANCGI:   Ptr(EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}),
		TAI:         Ptr(TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7}),
		RequestType: Ptr(RequestType{EventType: EventTypeChangeOfServeCell, ReportArea: ReportAreaECGI}),
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

	if deref(out.EUTRANCGI) != deref(in.EUTRANCGI) || deref(out.TAI) != deref(in.TAI) {
		t.Fatalf("location: cgi=%+v tai=%+v", out.EUTRANCGI, out.TAI)
	}

	if deref(out.RequestType) != deref(in.RequestType) {
		t.Fatalf("request type = %+v, want %+v", out.RequestType, in.RequestType)
	}
}

// §9.1.12.3 assigns reject to the two UE IDs and ignore to E-UTRAN CGI, TAI
// and Request Type.
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

		out, err := ParseLocationReport(value)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		if out.EUTRANCGI != nil || out.TAI != nil || out.RequestType != nil {
			t.Fatalf("absent IEs decoded as present: %+v", out)
		}

		got := out.Diagnostics().IEs
		if len(got) != 3 {
			t.Fatalf("diagnostics = %v, want 3 entries", got)
		}

		for i, want := range []ProtocolIEID{idEUTRANCGI, idTAI, idRequestType} {
			if got[i].IEID != want || got[i].IECriticality != CriticalityIgnore ||
				got[i].TypeOfError != TypeOfErrorMissing {
				t.Errorf("diagnostic %d = %+v, want %v missing/ignore", i, got[i], want)
			}
		}
	})

	t.Run("missing reject-criticality IE rejects the procedure", func(t *testing.T) {
		value := encode(t, []ieField{
			{id: idMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		})

		var ase *AbstractSyntaxError
		if _, err := ParseLocationReport(value); !errors.As(err, &ase) {
			t.Fatalf("error = %v, want *AbstractSyntaxError", err)
		}

		if len(ase.IEs) != 1 || ase.IEs[0].IEID != idENBUES1APID || ase.IEs[0].TypeOfError != TypeOfErrorMissing {
			t.Fatalf("diagnostics = %v, want [eNB-UE-S1AP-ID missing]", ase.IEs)
		}

		mmeID, enbID := ase.UEIDs()
		if mmeID == nil || *mmeID != 1 || enbID != nil {
			t.Fatalf("UEIDs() = %v, %v, want 1, nil", mmeID, enbID)
		}
	})
}
