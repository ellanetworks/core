// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

// TestUnknownIERoundTrip checks a ProtocolIE the message type does not model is
// preserved through Marshal/Parse and surfaced by UnknownIEs (TS 36.413),
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
// (TS 36.413) round-trips on a message — Error Indication — where it was
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

// TestUnhandledCriticalIEs checks that an unmodeled IE marked reject is
// surfaced for the receiver to act on (TS 36.413 §10.3.4.2), while ignore and
// notify ones are not, and that the parse itself still succeeds — deciding is
// the receiving node's job.
func TestUnhandledCriticalIEs(t *testing.T) {
	const (
		rejectID ProtocolIEID = 301
		ignoreID ProtocolIEID = 302
		notifyID ProtocolIEID = 303
	)

	m := &S1SetupRequest{
		GlobalENBID:  GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{Kind: ENBIDMacro, Value: 1}},
		SupportedTAs: SupportedTAs{{TAC: 1, BroadcastPLMNs: BPLMNs{{0x00, 0xf1, 0x10}}}},
		unmodeledIEs: unmodeledIEs{unknownIEs: []rawIE{
			{id: ignoreID, crit: CriticalityIgnore, value: []byte{0x01}},
			{id: rejectID, crit: CriticalityReject, value: []byte{0x02}},
			{id: notifyID, crit: CriticalityNotify, value: []byte{0x03}},
		}},
	}

	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseS1SetupRequest(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatalf("parse must succeed; the receiver decides: %v", err)
	}

	got := out.UnhandledCriticalIEs()
	if len(got) != 1 || got[0] != rejectID {
		t.Fatalf("UnhandledCriticalIEs() = %v, want [%v]", got, rejectID)
	}
	// All three survive for re-encode regardless of criticality.
	if len(out.UnknownIEs()) != 3 {
		t.Fatalf("UnknownIEs() = %d entries, want 3", len(out.UnknownIEs()))
	}
}

func TestUnhandledCriticalIEsNilWhenNone(t *testing.T) {
	m := &S1SetupRequest{
		unmodeledIEs: unmodeledIEs{unknownIEs: []rawIE{
			{id: 400, crit: CriticalityIgnore, value: []byte{0x01}},
		}},
	}

	if got := m.UnhandledCriticalIEs(); got != nil {
		t.Fatalf("UnhandledCriticalIEs() = %v, want nil", got)
	}
}
