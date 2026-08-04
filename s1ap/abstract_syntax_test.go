// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// As a peer would send it, without the rules the encoder normally enforces.
func container(t *testing.T, fields ...ieField) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(false)

	if err := encodeIEContainer(w, per.Aligned, fields); err != nil {
		t.Fatalf("encode container: %v", err)
	}

	return perBytes(w)
}

func uplinkNASFields() []ieField {
	return []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: Ptr(MMEUES1APID(7))},
		{id: idENBUES1APID, crit: CriticalityReject, val: Ptr(ENBUES1APID(9))},
		{id: idNASPDU, crit: CriticalityReject, val: Ptr(NASPDU{0x01})},
		{id: idEUTRANCGI, crit: CriticalityIgnore, val: Ptr(EUTRANCGI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}})},
		{id: idTAI, crit: CriticalityIgnore, val: Ptr(TAI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}, TAC: TAC(1)})},
	}
}

// TS 36.413 §10.3.6: an IE carried twice is a falsely constructed message.
func TestDuplicateIERejected(t *testing.T) {
	fields := uplinkNASFields()
	value := container(t, append(fields, fields[3])...)

	_, err := ParseUplinkNASTransport(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage {
		t.Errorf("cause = %s, want falsely-constructed-message", ase.Cause)
	}
}

// TS 36.413 §10.3.6: IEs outside the order §9.1 defines are equally a falsely
// constructed message.
func TestIEsOutOfOrderRejected(t *testing.T) {
	fields := uplinkNASFields()
	fields[0], fields[1] = fields[1], fields[0]

	_, err := ParseUplinkNASTransport(container(t, fields...))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage {
		t.Errorf("cause = %s, want falsely-constructed-message", ase.Cause)
	}
}

// §10.3.6 considers only IEs the receiver's version defines.
func TestUnknownIEDoesNotBreakOrdering(t *testing.T) {
	fields := uplinkNASFields()
	unknown := ieField{id: ProtocolIEID(60000), crit: CriticalityIgnore, raw: []byte{0x01, 0x02}}
	fields = append([]ieField{fields[0], fields[1], unknown}, fields[2:]...)

	msg, err := ParseUplinkNASTransport(container(t, fields...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got := msg.UnknownIEs(); len(got) != 1 || got[0].ID != 60000 {
		t.Errorf("unknown IEs = %v, want the id 60000 IE preserved", got)
	}
}

// TS 36.413 §10.3.5: an absent ignore-criticality IE is reported, and the
// message is still delivered with the field left absent.
func TestAbsentIgnoreIEIsDelivered(t *testing.T) {
	fields := uplinkNASFields()

	msg, err := ParseUplinkNASTransport(container(t, fields[:4]...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.TAI != nil {
		t.Errorf("TAI = %v, want nil", msg.TAI)
	}

	diag := msg.Diagnostics()
	if len(diag.IEs) != 1 || diag.IEs[0].ID != idTAI || diag.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Fatalf("diagnostics = %+v, want TAI reported missing", diag.IEs)
	}

	if diag.ReportRequired() {
		t.Error("ReportRequired() = true, want false for ignore criticality")
	}
}

// TS 36.413 §10.3.5: an absent reject-criticality IE stops delivery, and the
// IEs that did arrive stay reachable so the rejection can be addressed.
func TestAbsentRejectIEStopsDelivery(t *testing.T) {
	fields := uplinkNASFields()

	_, err := ParseUplinkNASTransport(container(t, fields[0], fields[3], fields[4]))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorReject {
		t.Errorf("cause = %s, want abstract-syntax-error-reject", ase.Cause)
	}

	mmeID, enbID := ase.UEIDs()
	if mmeID == nil || *mmeID != 7 {
		t.Errorf("MME-UE-S1AP-ID = %v, want 7", mmeID)
	}

	if enbID != nil {
		t.Errorf("eNB-UE-S1AP-ID = %v, want nil", enbID)
	}

	want := map[ProtocolIEID]bool{idENBUES1APID: true, idNASPDU: true}
	for _, ie := range ase.IEs {
		if !want[ie.IEID] || ie.TypeOfError != TypeOfErrorMissing {
			t.Errorf("unexpected diagnostic %+v", ie)
		}
	}

	if len(ase.IEs) != len(want) {
		t.Errorf("diagnostics = %+v, want %d entries", ase.IEs, len(want))
	}
}

// TS 36.413 §9.1.2.1 obliges the sender even where §10.3.5 lets a receiver
// carry on, so encoding an unset required IE fails.
func TestEncodeRejectsUnsetRequiredIE(t *testing.T) {
	msg := &UplinkNASTransport{
		MMEUES1APID: 7,
		ENBUES1APID: 9,
		NASPDU:      NASPDU{0x01},
		EUTRANCGI:   Ptr(EUTRANCGI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}}),
	}

	if _, err := msg.Marshal(); err == nil {
		t.Fatal("Marshal() = nil error, want a required-IE error for the absent TAI")
	}
}

// Recording is bounded, since a peer chooses how many IEs to send.
func TestDiagnosticsAreBounded(t *testing.T) {
	fields := uplinkNASFields()
	for i := range maxDiagnosticIEs + 10 {
		fields = append(fields, ieField{
			id: ProtocolIEID(40000 + i), crit: CriticalityIgnore, raw: []byte{0x00},
		})
	}

	msg, err := ParseUplinkNASTransport(container(t, fields...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	diag := msg.Diagnostics()
	if len(diag.IEs) > maxDiagnosticIEs {
		t.Errorf("recorded %d diagnostics, want at most %d", len(diag.IEs), maxDiagnosticIEs)
	}

	if !diag.Truncated {
		t.Error("Truncated = false, want true")
	}

	if got := len(msg.UnknownIEs()); got > maxPreservedIEs {
		t.Errorf("preserved %d unknown IEs, want at most %d", got, maxPreservedIEs)
	}
}

// Padding must not push the UE S1AP IDs out of what the rejection retains, or
// the ERROR INDICATION cannot name its association (§8.7.2.2).
func TestUEIDsSurvivePadding(t *testing.T) {
	fields := uplinkNASFields()

	var padded []ieField
	for i := range maxPreservedIEs * 2 {
		padded = append(padded, ieField{
			id: ProtocolIEID(40000 + i), crit: CriticalityIgnore, raw: []byte{0x00},
		})
	}

	// The UE IDs arrive after the padding, and the mandatory NAS-PDU is absent.
	padded = append(padded, fields[0], fields[1], fields[4])

	_, err := ParseUplinkNASTransport(container(t, padded...))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	mmeID, enbID := ase.UEIDs()
	if mmeID == nil || *mmeID != 7 || enbID == nil || *enbID != 9 {
		t.Fatalf("UEIDs() = (%v, %v), want (7, 9)", mmeID, enbID)
	}
}

// Octets that are not a decodable PER encoding are a transfer syntax error,
// distinct from a message that decodes but must be rejected (TS 36.413 §10.2).
func TestTransferSyntaxError(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
	}{
		{"empty", nil},
		{"truncated container", []byte{0x00, 0x00, 0x01}},
		{"IE value past the end", []byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x40, 0x7f}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseUplinkNASTransport(tt.value)

			var tse *TransferSyntaxError
			if !errors.As(err, &tse) {
				t.Fatalf("error = %v, want *TransferSyntaxError", err)
			}

			if tse.Procedure != ProcUplinkNASTransport {
				t.Errorf("procedure = %s, want ProcUplinkNASTransport", tse.Procedure)
			}

			if tse.Unwrap() == nil {
				t.Error("Unwrap() = nil, want the underlying decode error")
			}
		})
	}
}

// A declared IE count the remaining octets cannot hold must be refused before
// anything is allocated for it.
func TestOversizedIECountRejected(t *testing.T) {
	_, err := ParseUplinkNASTransport([]byte{0x00, 0xff, 0xff, 0x00})

	var tse *TransferSyntaxError
	if !errors.As(err, &tse) {
		t.Fatalf("error = %v, want *TransferSyntaxError", err)
	}
}

// PATH SWITCH REQUEST carries the source MME-UE-S1AP-ID instead of the plain
// one (§8.7.2.2).
func TestUEIDsFromPathSwitchRequest(t *testing.T) {
	value := container(t,
		ieField{id: idENBUES1APID, crit: CriticalityReject, val: Ptr(ENBUES1APID(9))},
		ieField{id: idSourceMMEUES1APID, crit: CriticalityReject, val: Ptr(MMEUES1APID(7))},
	)

	_, err := ParsePathSwitchRequest(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	mmeID, enbID := ase.UEIDs()
	if mmeID == nil || *mmeID != 7 || enbID == nil || *enbID != 9 {
		t.Fatalf("UEIDs() = (%v, %v), want (7, 9)", mmeID, enbID)
	}
}

// A notify entry must survive the diagnostics bound or the §10.3.4.2 report
// carries no IE.
func TestNotifySurvivesTruncation(t *testing.T) {
	fields := uplinkNASFields()
	for i := range maxDiagnosticIEs + 10 {
		fields = append(fields, ieField{
			id: ProtocolIEID(40000 + i), crit: CriticalityIgnore, raw: []byte{0x00},
		})
	}

	fields = append(fields, ieField{id: ProtocolIEID(50000), crit: CriticalityNotify, raw: []byte{0x00}})

	msg, err := ParseUplinkNASTransport(container(t, fields...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	diag := msg.Diagnostics()
	if !diag.ReportRequired() {
		t.Fatal("ReportRequired() = false, want true")
	}

	report := diag.Report()
	if len(report) != 1 || report[0].IEID != 50000 {
		t.Fatalf("Report() = %+v, want the notify IE", report)
	}

	if len(diag.IEs) > maxDiagnosticIEs {
		t.Errorf("recorded %d diagnostics, want at most %d", len(diag.IEs), maxDiagnosticIEs)
	}
}
