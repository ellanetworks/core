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

	// Every IE is optional, so an empty one is decodable; the application guard
	// decides what to do with it.
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

// §8.7.2.2 requires at least one of Cause and Criticality Diagnostics, and
// §10.3.3 binds the sender.
func TestErrorIndicationRefusesEmptyOnSend(t *testing.T) {
	if _, err := (&ErrorIndication{}).Marshal(); err == nil {
		t.Fatal("Marshal() = nil error, want a refusal for a message with neither IE")
	}
}

// An eNB that knows only the S-TMSI names the UE with it.
func TestErrorIndicationSTMSIRoundTrip(t *testing.T) {
	sent := &ErrorIndication{
		Cause: &Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified},
		STMSI: &STMSI{MMEC: 0x2a, MTMSI: 0xdeadbeef},
	}

	raw, err := sent.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got, err := ParseErrorIndication(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.STMSI == nil {
		t.Fatal("STMSI decoded to nil")
	}

	if *got.STMSI != *sent.STMSI {
		t.Errorf("STMSI = %+v, want %+v", *got.STMSI, *sent.STMSI)
	}
}

// An all-zero S-TMSI is a legal identity, so absent must stay absent.
func TestErrorIndicationSTMSIStaysAbsent(t *testing.T) {
	raw, err := (&ErrorIndication{Cause: &Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}}).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseErrorIndication(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.STMSI != nil {
		t.Errorf("STMSI = %+v, want nil", *got.STMSI)
	}
}
