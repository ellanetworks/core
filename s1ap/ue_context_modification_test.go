// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"
)

func TestUEContextModificationRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		in := &UEContextModificationRequest{
			MMEUES1APID:               42,
			ENBUES1APID:               7,
			UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{DL: 200_000_000, UL: 100_000_000},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		if b[0] != 0x00 || b[1] != byte(ProcUEContextModification) {
			t.Fatalf("envelope prefix = % x, want 00 15 (initiatingMessage / UE Context Modification)", b[:2])
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseUEContextModificationRequest(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.MMEUES1APID != 42 || out.ENBUES1APID != 7 || out.UEAggregateMaximumBitRate == nil ||
			*out.UEAggregateMaximumBitRate != *in.UEAggregateMaximumBitRate {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})

	t.Run("Request without UE-AMBR", func(t *testing.T) {
		in := &UEContextModificationRequest{MMEUES1APID: 1, ENBUES1APID: 2}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, _ := Unmarshal(b)

		out, err := ParseUEContextModificationRequest(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.UEAggregateMaximumBitRate != nil {
			t.Fatalf("UE-AMBR = %+v, want absent", out.UEAggregateMaximumBitRate)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := &UEContextModificationResponse{MMEUES1APID: Ptr(MMEUES1APID(42)), ENBUES1APID: Ptr(ENBUES1APID(7))}

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

		if deref(out.MMEUES1APID) != 42 || deref(out.ENBUES1APID) != 7 {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		in := &UEContextModificationFailure{
			MMEUES1APID: Ptr(MMEUES1APID(42)),
			ENBUES1APID: Ptr(ENBUES1APID(7)),
			Cause:       Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 0}),
		}

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

		if deref(out.MMEUES1APID) != 42 || deref(out.ENBUES1APID) != 7 || deref(out.Cause) != deref(in.Cause) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})
}
