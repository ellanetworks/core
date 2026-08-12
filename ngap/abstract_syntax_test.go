// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

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

func ngSetupFields() []ieField {
	src := goldRequest()

	return []ieField{
		{id: IDGlobalRANNodeID, crit: CriticalityReject, val: &src.GlobalRANNodeID},
		{id: IDRANNodeName, crit: CriticalityIgnore, val: Name("ella-gnb")},
		{id: IDSupportedTAList, crit: CriticalityReject, val: src.SupportedTAList},
		{id: IDDefaultPagingDRX, crit: CriticalityIgnore, val: PagingDRXv128},
	}
}

// TS 38.413 §10.3.6: an IE carried twice is a falsely constructed message.
func TestDuplicateIERejected(t *testing.T) {
	fields := ngSetupFields()
	value := container(t, append(fields, fields[3])...)

	_, err := ParseNGSetupRequest(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage {
		t.Errorf("cause = %s, want falsely-constructed-message", ase.Cause)
	}
}

// TS 38.413 §10.3.6: IEs outside the order §9.2.6.1 defines are equally a
// falsely constructed message.
func TestIEsOutOfOrderRejected(t *testing.T) {
	fields := ngSetupFields()
	fields[0], fields[1] = fields[1], fields[0]

	_, err := ParseNGSetupRequest(container(t, fields...))

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
	fields := ngSetupFields()
	unknown := ieField{id: ProtocolIEID(60000), crit: CriticalityIgnore, raw: []byte{0x01, 0x02}}
	fields = append([]ieField{fields[0], fields[1], unknown}, fields[2:]...)

	msg, err := ParseNGSetupRequest(container(t, fields...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got := msg.UnknownIEs(); len(got) != 1 || got[0].ID != 60000 {
		t.Errorf("unknown IEs = %v, want the id 60000 IE preserved", got)
	}
}

// TS 38.413 §10.3.5: an absent ignore-criticality IE is reported, and the
// message is still delivered with the field left absent.
func TestAbsentIgnoreIEIsDelivered(t *testing.T) {
	fields := ngSetupFields()

	msg, err := ParseNGSetupRequest(container(t, fields[:3]...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.DefaultPagingDRX != nil {
		t.Errorf("DefaultPagingDRX = %v, want nil for an absent IE", *msg.DefaultPagingDRX)
	}

	diag := msg.Diagnostics()
	if len(diag.IEs) != 1 || diag.IEs[0].ID != IDDefaultPagingDRX ||
		diag.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Fatalf("diagnostics = %+v", diag.IEs)
	}
}

// TS 38.413 §10.3.5: an absent reject-criticality IE stops the message from
// reaching the application, so no message is returned.
func TestAbsentRejectIEStopsDelivery(t *testing.T) {
	fields := ngSetupFields()

	msg, err := ParseNGSetupRequest(container(t, fields[1:]...))
	if msg != nil {
		t.Error("a message was returned for a rejected procedure")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorReject {
		t.Errorf("cause = %s, want abstract-syntax-error-reject", ase.Cause)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != IDGlobalRANNodeID {
		t.Fatalf("reported IEs = %+v", ase.IEs)
	}
}

// TS 38.413 §9.1.1 obliges the sender even where §10.3.5 lets a receiver
// carry on, so encoding an unset required IE fails.
func TestEncodeRejectsUnsetRequiredIE(t *testing.T) {
	msg := &NGSetupRequest{
		GlobalRANNodeID:  goldRequest().GlobalRANNodeID,
		RANNodeName:      Ptr("ella-gnb"),
		DefaultPagingDRX: Ptr(PagingDRXv128),
	}

	if _, err := msg.Marshal(); err == nil {
		t.Fatal("Marshal() = nil error, want a required-IE error for the absent SupportedTAList")
	}
}

// Recording is bounded, since a peer chooses how many IEs to send.
func TestDiagnosticsAreBounded(t *testing.T) {
	fields := ngSetupFields()
	for i := range maxDiagnosticIEs + 10 {
		fields = append(fields, ieField{
			id: ProtocolIEID(40000 + i), crit: CriticalityIgnore, raw: []byte{0x00},
		})
	}

	msg, err := ParseNGSetupRequest(container(t, fields...))
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

// Octets that are not a decodable PER encoding are a transfer syntax error,
// distinct from a message that decodes but must be rejected (TS 38.413 §10.2).
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
			_, err := ParseNGSetupRequest(tt.value)

			var tse *TransferSyntaxError
			if !errors.As(err, &tse) {
				t.Fatalf("error = %v, want *TransferSyntaxError", err)
			}

			if tse.Procedure != ProcNGSetup {
				t.Errorf("procedure = %s, want ProcNGSetup", tse.Procedure)
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
	_, err := ParseNGSetupRequest([]byte{0x00, 0xff, 0xff, 0x00})

	var tse *TransferSyntaxError
	if !errors.As(err, &tse) {
		t.Fatalf("error = %v, want *TransferSyntaxError", err)
	}
}

// A notify entry must survive the diagnostics bound or the §10.3.4.2 report
// carries no IE. No IE table assigns notify, but a peer chooses what it stamps.
func TestNotifySurvivesTruncation(t *testing.T) {
	fields := ngSetupFields()
	for i := range maxDiagnosticIEs + 10 {
		fields = append(fields, ieField{
			id: ProtocolIEID(40000 + i), crit: CriticalityIgnore, raw: []byte{0x00},
		})
	}

	fields = append(fields, ieField{id: ProtocolIEID(50000), crit: CriticalityNotify, raw: []byte{0x00}})

	msg, err := ParseNGSetupRequest(container(t, fields...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	diag := msg.Diagnostics()
	if !diag.ReportRequired() {
		t.Fatal("ReportRequired() = false, want true")
	}

	report := diag.Report()
	if len(report) != 1 || report[0].IEID != 50000 || report[0].IECriticality != CriticalityNotify {
		t.Fatalf("Report() = %+v, want the notify entry", report)
	}
}

// §8.4.4.2. NG Setup is node-level, so the decoded IEs are supplied directly
// until a UE-associated message is modeled.
func TestUEIDsRecoverTheAssociation(t *testing.T) {
	encode := func(t *testing.T, m per.Marshaler) []byte {
		t.Helper()

		w := per.NewWriter()
		if err := m.MarshalPER(w, per.Aligned); err != nil {
			t.Fatal(err)
		}

		return perBytes(w)
	}

	amf := encode(t, Ptr(AMFUENGAPID(7)))
	ran := encode(t, Ptr(RANUENGAPID(9)))

	tests := []struct {
		name    string
		decoded []RawIE
		wantAMF *AMFUENGAPID
		wantRAN *RANUENGAPID
	}{
		{
			"canonical pair",
			[]RawIE{{ID: IDAMFUENGAPID, Value: amf}, {ID: IDRANUENGAPID, Value: ran}},
			Ptr(AMFUENGAPID(7)), Ptr(RANUENGAPID(9)),
		},
		{
			// PATH SWITCH REQUEST names the association by the source AMF id.
			"source AMF id",
			[]RawIE{{ID: IDSourceAMFUENGAPID, Value: amf}, {ID: IDRANUENGAPID, Value: ran}},
			Ptr(AMFUENGAPID(7)), Ptr(RANUENGAPID(9)),
		},
		{
			"RAN id only",
			[]RawIE{{ID: IDRANUENGAPID, Value: ran}},
			nil, Ptr(RANUENGAPID(9)),
		},
		{
			"neither arrived",
			[]RawIE{{ID: IDCause, Value: []byte{0x00}}},
			nil, nil,
		},
		{
			// A value that does not decode leaves the id absent, not zero.
			"undecodable id",
			[]RawIE{{ID: IDAMFUENGAPID, Value: nil}},
			nil, nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ase := &AbstractSyntaxError{Procedure: ProcNGSetup, decoded: tt.decoded}

			gotAMF, gotRAN := ase.UEIDs()

			if (gotAMF == nil) != (tt.wantAMF == nil) ||
				(gotAMF != nil && *gotAMF != *tt.wantAMF) {
				t.Errorf("AMF id = %v, want %v", gotAMF, tt.wantAMF)
			}

			if (gotRAN == nil) != (tt.wantRAN == nil) ||
				(gotRAN != nil && *gotRAN != *tt.wantRAN) {
				t.Errorf("RAN id = %v, want %v", gotRAN, tt.wantRAN)
			}
		})
	}
}

// Padding must not push the UE NGAP IDs out of what the rejection retains, or
// the ERROR INDICATION cannot name its association (§8.4.4.2).
func TestModeledIEsSurvivePadding(t *testing.T) {
	fields := ngSetupFields()

	var padded []ieField
	for i := range maxPreservedIEs * 2 {
		padded = append(padded, ieField{
			id: ProtocolIEID(40000 + i), crit: CriticalityIgnore, raw: []byte{0x00},
		})
	}

	// The modeled IEs arrive after the padding, and the mandatory
	// SupportedTAList is absent.
	padded = append(padded, fields[0], fields[3])

	_, err := ParseNGSetupRequest(container(t, padded...))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	var found bool

	for _, ie := range ase.decoded {
		if ie.ID == IDGlobalRANNodeID {
			found = true
		}
	}

	if !found {
		t.Errorf("decoded IEs = %+v, want the GlobalRANNodeID retained past the padding", ase.decoded)
	}
}
