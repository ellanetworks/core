// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestUEContextReleaseRoundTrips(t *testing.T) {
	cause := Cause{Group: CauseGroupRadioNetwork, Value: 0}

	t.Run("Command pair", func(t *testing.T) {
		in := &UEContextReleaseCommand{
			UES1APIDs: UES1APIDs{MMEUES1APID: 1, ENBUES1APID: 7, Pair: true},
			Cause:     cause,
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
		if !ok || im.ProcedureCode != ProcUEContextRelease {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextReleaseCommand(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if !out.UES1APIDs.Pair || out.UES1APIDs.MMEUES1APID != 1 || out.UES1APIDs.ENBUES1APID != 7 || out.Cause != cause {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Command bare MME id", func(t *testing.T) {
		in := &UEContextReleaseCommand{UES1APIDs: UES1APIDs{MMEUES1APID: 42}, Cause: cause}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		out, err := ParseUEContextReleaseCommand(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.UES1APIDs.Pair || out.UES1APIDs.MMEUES1APID != 42 {
			t.Fatalf("mismatch: %+v", out.UES1APIDs)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		in := &UEContextReleaseComplete{MMEUES1APID: 1, ENBUES1APID: 7}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcUEContextRelease {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParseUEContextReleaseComplete(so.Value)
		if err != nil || out.MMEUES1APID != 1 || out.ENBUES1APID != 7 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Request", func(t *testing.T) {
		in := &UEContextReleaseRequest{MMEUES1APID: 1, ENBUES1APID: 7, Cause: cause}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcUEContextReleaseRequest {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextReleaseRequest(im.Value)
		if err != nil || out.MMEUES1APID != 1 || out.ENBUES1APID != 7 || out.Cause != cause {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}
