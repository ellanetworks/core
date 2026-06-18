// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

// TestUnknownIERoundTrip checks a ProtocolIE the message type does not model is
// preserved through Marshal/Parse and surfaced by UnknownIEs (TS 36.413 §9.3),
// so a present IE is never silently dropped. UECapabilityInfoIndication is used
// because it previously discarded unmodeled IEs entirely.
func TestUnknownIERoundTrip(t *testing.T) {
	const unknownID ProtocolIEID = 300

	unknownVal := []byte{0xde, 0xad, 0xbe, 0xef}

	in := &UECapabilityInfoIndication{
		MMEUES1APID:       1,
		ENBUES1APID:       2,
		UERadioCapability: []byte{0xaa, 0xbb},
		unmodeledIEs:      unmodeledIEs{unknownIEs: []rawIE{{id: unknownID, crit: CriticalityNotify, value: unknownVal}}},
	}

	pduBytes, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(pduBytes)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok {
		t.Fatalf("PDU = %T, want *InitiatingMessage", pdu)
	}

	out, err := ParseUECapabilityInfoIndication(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	raw := out.UnknownIEs()
	if len(raw) != 1 {
		t.Fatalf("UnknownIEs len = %d, want 1", len(raw))
	}

	if raw[0].ID != unknownID || raw[0].Criticality != CriticalityNotify || !bytes.Equal(raw[0].Value, unknownVal) {
		t.Fatalf("unknown IE = %+v, want id=%d crit=notify value=%x", raw[0], unknownID, unknownVal)
	}
}

// TestUnknownIEsNilWhenNone checks UnknownIEs reports nil when every IE on the
// wire is modeled, so the accessor distinguishes "none" from "present but raw".
func TestUnknownIEsNilWhenNone(t *testing.T) {
	in := &UEContextReleaseCommand{
		UES1APIDs: UES1APIDs{MMEUES1APID: 1, ENBUES1APID: 2, Pair: true},
		Cause:     Cause{Group: CauseGroupNAS, Value: 0},
	}

	pduBytes, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(pduBytes)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseUEContextReleaseCommand(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if got := out.UnknownIEs(); got != nil {
		t.Fatalf("UnknownIEs = %+v, want nil", got)
	}
}

// TestErrorIndicationCriticalityDiagnostics checks the CriticalityDiagnostics IE
// (TS 36.413 §9.2.1.4) round-trips on a message — Error Indication — where it was
// previously dropped.
func TestErrorIndicationCriticalityDiagnostics(t *testing.T) {
	pc := ProcInitialContextSetup
	tm := TriggeringSuccessfulOutcome
	crit := CriticalityReject

	in := &ErrorIndication{
		CriticalityDiagnostics: &CriticalityDiagnostics{
			ProcedureCode:        &pc,
			TriggeringMessage:    &tm,
			ProcedureCriticality: &crit,
			IEsCriticalityDiagnostics: []CriticalityDiagnosticsIEItem{
				{IECriticality: CriticalityReject, IEID: idCause, TypeOfError: TypeOfErrorMissing},
			},
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseErrorIndication(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	cd := out.CriticalityDiagnostics
	if cd == nil {
		t.Fatal("CriticalityDiagnostics is nil")
	}

	if cd.ProcedureCode == nil || *cd.ProcedureCode != pc ||
		cd.TriggeringMessage == nil || *cd.TriggeringMessage != tm ||
		cd.ProcedureCriticality == nil || *cd.ProcedureCriticality != crit {
		t.Fatalf("scalar mismatch: %+v", cd)
	}

	if len(cd.IEsCriticalityDiagnostics) != 1 ||
		cd.IEsCriticalityDiagnostics[0].IEID != idCause ||
		cd.IEsCriticalityDiagnostics[0].TypeOfError != TypeOfErrorMissing {
		t.Fatalf("IE list mismatch: %+v", cd.IEsCriticalityDiagnostics)
	}
}
