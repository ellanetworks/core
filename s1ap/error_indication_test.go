// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestErrorIndicationRoundTrips(t *testing.T) {
	mme := MMEUES1APID(1)
	enb := ENBUES1APID(7)
	cause := Cause{Group: CauseGroupRadioNetwork, Value: 0}

	t.Run("full", func(t *testing.T) {
		in := &ErrorIndication{MMEUES1APID: &mme, ENBUES1APID: &enb, Cause: &cause}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcErrorIndication {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseErrorIndication(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.MMEUES1APID == nil || *out.MMEUES1APID != mme ||
			out.ENBUES1APID == nil || *out.ENBUES1APID != enb ||
			out.Cause == nil || *out.Cause != cause {
			t.Fatalf("mismatch: %+v", out)
		}
	})

	t.Run("empty", func(t *testing.T) {
		b, _ := (&ErrorIndication{}).Marshal()

		pdu, _ := Unmarshal(b)

		out, err := ParseErrorIndication(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.MMEUES1APID != nil || out.ENBUES1APID != nil || out.Cause != nil {
			t.Fatalf("expected all-absent IEs, got %+v", out)
		}
	})
}
