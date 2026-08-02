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

	// Every IE is optional in the ASN.1 (§9.1.8.6), so a peer can still put an
	// empty one on the wire. The receiver must decode it rather than fall over;
	// the application-level guard then decides what to do with it.
	t.Run("empty", func(t *testing.T) {
		out, err := ParseErrorIndication(container(t))
		if err != nil {
			t.Fatal(err)
		}

		if out.MMEUES1APID != nil || out.ENBUES1APID != nil || out.Cause != nil {
			t.Fatalf("expected all-absent IEs, got %+v", out)
		}
	})
}

// TS 36.413 §8.7.2.2: "The ERROR INDICATION message shall contain at least
// either the Cause IE or the Criticality Diagnostics IE." §10.3.3 binds the
// sender, so encoding one that carries neither must fail.
func TestErrorIndicationRefusesEmptyOnSend(t *testing.T) {
	if _, err := (&ErrorIndication{}).Marshal(); err == nil {
		t.Fatal("Marshal() = nil error, want a refusal for a message with neither IE")
	}
}
