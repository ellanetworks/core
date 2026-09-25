// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "testing"

func TestUEContextModificationRequestRoundTrips(t *testing.T) {
	in := &UEContextModificationRequest{
		AMFUENGAPID:               42,
		RANUENGAPID:               1,
		UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{DL: 200_000_000, UL: 100_000_000},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcUEContextModification || im.Criticality != CriticalityReject {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseUEContextModificationRequest(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != 42 || out.RANUENGAPID != 1 || out.UEAggregateMaximumBitRate == nil ||
		*out.UEAggregateMaximumBitRate != *in.UEAggregateMaximumBitRate {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

func TestUEContextModificationResponseRoundTrips(t *testing.T) {
	in := &UEContextModificationResponse{AMFUENGAPID: Ptr(AMFUENGAPID(42)), RANUENGAPID: Ptr(RANUENGAPID(1))}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcUEContextModification {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseUEContextModificationResponse(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if deref(out.AMFUENGAPID) != 42 || deref(out.RANUENGAPID) != 1 {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

func TestUEContextModificationFailureRoundTrips(t *testing.T) {
	cause := Cause{Group: CauseGroupRadioNetwork, Value: 0}
	in := &UEContextModificationFailure{AMFUENGAPID: Ptr(AMFUENGAPID(42)), RANUENGAPID: Ptr(RANUENGAPID(1)), Cause: &cause}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcUEContextModification {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseUEContextModificationFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if deref(out.AMFUENGAPID) != 42 || deref(out.RANUENGAPID) != 1 || out.Cause == nil || *out.Cause != cause {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}
