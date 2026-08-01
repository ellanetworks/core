// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// idNBIoTDefaultPagingDRX is an NG SETUP REQUEST IE this version does not
// model (TS 38.413 §9.2.6.1). It is ignore criticality, so §10.3.4.2 says the
// receiver carries on.
const idNBIoTDefaultPagingDRX ProtocolIEID = 204

// encodeWithExtraIE writes an NG SETUP REQUEST carrying one IE the table does
// not model, at the given criticality.
func encodeWithExtraIE(t *testing.T, crit Criticality, id ProtocolIEID) []byte {
	t.Helper()

	src := goldRequest()

	w := per.NewWriter()
	w.WriteBit(false)

	fields := []ieField{
		{id: idGlobalRANNodeID, crit: CriticalityReject, val: &src.GlobalRANNodeID},
		{id: idSupportedTAList, crit: CriticalityReject, val: src.SupportedTAList},
		{id: idDefaultPagingDRX, crit: CriticalityIgnore, val: PagingDRXv128},
		{id: id, crit: crit, raw: []byte{0xde, 0xad}},
	}

	if err := encodeIEContainer(w, per.Aligned, fields); err != nil {
		t.Fatalf("encode: %v", err)
	}

	return perBytes(w)
}

// TS 38.413 §10.3.4.2: an IE the receiver does not comprehend, marked ignore,
// is reported and the message is still delivered. Preserving it verbatim keeps
// a re-encode faithful to what the peer sent.
func TestUnmodeledIgnoreIEIsPreserved(t *testing.T) {
	value := encodeWithExtraIE(t, CriticalityIgnore, idNBIoTDefaultPagingDRX)

	req, err := ParseNGSetupRequest(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	d := req.Diagnostics()
	if d.Empty() {
		t.Fatal("diagnostics empty, want the unmodeled IE reported")
	}

	if got := d.IEs[0]; got.ID != idNBIoTDefaultPagingDRX ||
		got.Criticality != CriticalityIgnore ||
		got.TypeOfError != TypeOfErrorNotUnderstood {
		t.Errorf("diagnostic = %+v", got)
	}

	// §9.3.1.3 forbids reporting an ignore IE, and TS 38.413 defines no notify
	// criticality at all, so nothing is reportable.
	if d.ReportRequired() || d.Report() != nil {
		t.Errorf("ReportRequired() = %v, Report() = %+v, want false and nil", d.ReportRequired(), d.Report())
	}

	unknown := req.UnknownIEs()
	if len(unknown) != 1 || unknown[0].ID != idNBIoTDefaultPagingDRX ||
		!bytes.Equal(unknown[0].Value, []byte{0xde, 0xad}) {
		t.Fatalf("UnknownIEs() = %+v", unknown)
	}

	// Re-encoding puts the preserved IE back on the wire after the modeled ones.
	out, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(out)
	if err != nil {
		t.Fatal(err)
	}

	r := per.NewReader(pdu.value())
	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	last := fields[len(fields)-1]
	if last.id != idNBIoTDefaultPagingDRX || !bytes.Equal(last.value, []byte{0xde, 0xad}) {
		t.Errorf("re-encoded trailing IE = %+v, want the preserved one", last)
	}
}

// TS 38.413 §10.3.4.2: the same IE marked reject stops the procedure.
func TestUnmodeledRejectIEIsRejected(t *testing.T) {
	value := encodeWithExtraIE(t, CriticalityReject, idNBIoTDefaultPagingDRX)

	msg, err := ParseNGSetupRequest(value)
	if msg != nil {
		t.Error("a message was returned for a rejected procedure")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %v, want *AbstractSyntaxError", err)
	}

	want := Cause{Group: CauseGroupProtocol, Value: CauseProtocolAbstractSyntaxErrorReject}
	if ase.Cause != want {
		t.Errorf("cause = %v, want %v", ase.Cause, want)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idNBIoTDefaultPagingDRX ||
		ase.IEs[0].TypeOfError != TypeOfErrorNotUnderstood {
		t.Errorf("reported IEs = %+v", ase.IEs)
	}
}

// A message with nothing unmodeled returns no unknown IEs, so a caller can
// test the slice rather than its length.
func TestUnknownIEsNilWhenNone(t *testing.T) {
	req, err := ParseNGSetupRequest(container(t, ngSetupFields()...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got := req.UnknownIEs(); got != nil {
		t.Errorf("UnknownIEs() = %+v, want nil", got)
	}

	if !req.Diagnostics().Empty() {
		t.Errorf("Diagnostics() = %+v, want empty", req.Diagnostics())
	}
}

// A preserved IE keeps the criticality the peer stamped, not the one this
// version would have chosen: re-emitting it under a different criticality
// would change what a downstream receiver is required to do with it.
func TestUnmodeledIECriticality(t *testing.T) {
	value := encodeWithExtraIE(t, CriticalityIgnore, idNBIoTDefaultPagingDRX)

	req, err := ParseNGSetupRequest(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got := req.UnknownIEs(); len(got) != 1 || got[0].Criticality != CriticalityIgnore {
		t.Fatalf("UnknownIEs() = %+v, want one ignore-criticality IE", got)
	}

	out, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(out)
	if err != nil {
		t.Fatal(err)
	}

	r := per.NewReader(pdu.value())
	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	last := fields[len(fields)-1]
	if last.id != idNBIoTDefaultPagingDRX || last.crit != CriticalityIgnore {
		t.Errorf("re-encoded IE = {%v, %v}, want the preserved id and criticality",
			last.id, last.crit)
	}
}
