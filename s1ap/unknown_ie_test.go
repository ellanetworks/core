// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"errors"
	"slices"
	"testing"
)

// A ProtocolIE the message type does not model must survive Marshal/Parse
// verbatim and surface through UnknownIEs (TS 36.413).
func TestUnknownIERoundTrip(t *testing.T) {
	const unknownID ProtocolIEID = 300

	unknownVal := []byte{0xde, 0xad, 0xbe, 0xef}

	in := &UECapabilityInfoIndication{
		MMEUES1APID:       1,
		ENBUES1APID:       2,
		UERadioCapability: []byte{0xaa, 0xbb},
		messageMeta:       messageMeta{unknownIEs: []rawIE{{id: unknownID, crit: CriticalityNotify, value: unknownVal}}},
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

func TestUnknownIEsNilWhenNone(t *testing.T) {
	in := &UEContextReleaseCommand{
		UES1APIDs: UES1APIDs{MMEUES1APID: 1, ENBUES1APID: 2, Pair: true},
		Cause:     Ptr(Cause{Group: CauseGroupNAS, Value: 0}),
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

// TS 36.413 §10.3.4.2: an unmodeled IE marked reject stops the procedure, while
// ignore and notify ones are carried past and preserved for re-encode.
func TestUnmodeledIECriticality(t *testing.T) {
	const (
		rejectID ProtocolIEID = 301
		ignoreID ProtocolIEID = 302
		notifyID ProtocolIEID = 303
	)

	setup := func(t *testing.T, unknown ...rawIE) []byte {
		t.Helper()

		m := &S1SetupRequest{
			GlobalENBID:      GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{Kind: ENBIDMacro, Value: 1}},
			SupportedTAs:     SupportedTAs{{TAC: 1, BroadcastPLMNs: BPLMNs{{0x00, 0xf1, 0x10}}}},
			DefaultPagingDRX: Ptr(PagingDRXv128),
			messageMeta:      messageMeta{unknownIEs: unknown},
		}

		raw, err := m.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(raw)
		if err != nil {
			t.Fatal(err)
		}

		return pdu.(*InitiatingMessage).Value
	}

	t.Run("reject rejects the procedure", func(t *testing.T) {
		value := setup(t,
			rawIE{id: ignoreID, crit: CriticalityIgnore, value: []byte{0x01}},
			rawIE{id: rejectID, crit: CriticalityReject, value: []byte{0x02}},
		)

		var ase *AbstractSyntaxError
		if _, err := ParseS1SetupRequest(value); !errors.As(err, &ase) {
			t.Fatalf("error = %v, want *AbstractSyntaxError", err)
		}

		if len(ase.IEs) != 1 || ase.IEs[0].IEID != rejectID ||
			ase.IEs[0].TypeOfError != TypeOfErrorNotUnderstood {
			t.Fatalf("diagnostics = %v, want [%v not-understood]", ase.IEs, rejectID)
		}

		if !slices.ContainsFunc(ase.Decoded, func(ie RawIE) bool { return ie.ID == idGlobalENBID }) {
			t.Fatalf("decoded IEs = %v, want Global-eNB-ID carried on the rejection", ase.Decoded)
		}
	})

	t.Run("ignore and notify are reported and preserved", func(t *testing.T) {
		value := setup(t,
			rawIE{id: ignoreID, crit: CriticalityIgnore, value: []byte{0x01}},
			rawIE{id: notifyID, crit: CriticalityNotify, value: []byte{0x03}},
		)

		out, err := ParseS1SetupRequest(value)
		if err != nil {
			t.Fatalf("parse must succeed; the receiver decides: %v", err)
		}

		got := out.Diagnostics()
		if len(got.IEs) != 2 || got.IEs[0].IEID != ignoreID || got.IEs[1].IEID != notifyID {
			t.Fatalf("diagnostics = %v, want [%v %v]", got.IEs, ignoreID, notifyID)
		}

		if !got.ReportRequired() {
			t.Error("ReportRequired() = false, want true for a notify IE")
		}

		if len(out.UnknownIEs()) != 2 {
			t.Fatalf("UnknownIEs() = %d entries, want 2", len(out.UnknownIEs()))
		}
	})

	t.Run("ignore alone needs no report", func(t *testing.T) {
		out, err := ParseS1SetupRequest(setup(t, rawIE{id: 400, crit: CriticalityIgnore, value: []byte{0x01}}))
		if err != nil {
			t.Fatal(err)
		}

		if out.Diagnostics().ReportRequired() {
			t.Error("ReportRequired() = true, want false for an ignore IE")
		}
	})
}
