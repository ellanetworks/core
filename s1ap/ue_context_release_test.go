// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestUEContextReleaseRoundTrips(t *testing.T) {
	cause := Cause{Group: CauseGroupRadioNetwork, Value: 0}

	t.Run("Command pair", func(t *testing.T) {
		in := &UEContextReleaseCommand{
			UES1APIDs: UES1APIDs{MMEUES1APID: 1, ENBUES1APID: 7, Pair: true},
			Cause:     &cause,
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

		if !out.UES1APIDs.Pair || out.UES1APIDs.MMEUES1APID != 1 || out.UES1APIDs.ENBUES1APID != 7 ||
			deref(out.Cause) != cause {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Command bare MME id", func(t *testing.T) {
		in := &UEContextReleaseCommand{UES1APIDs: UES1APIDs{MMEUES1APID: 42}, Cause: &cause}

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
		in := &UEContextReleaseComplete{MMEUES1APID: Ptr(MMEUES1APID(1)), ENBUES1APID: Ptr(ENBUES1APID(7))}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcUEContextRelease {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParseUEContextReleaseComplete(so.Value)
		if err != nil || deref(out.MMEUES1APID) != 1 || deref(out.ENBUES1APID) != 7 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("CompleteWithUserLocation", func(t *testing.T) {
		plmn := PLMNIdentity{0x00, 0xf1, 0x10}
		in := &UEContextReleaseComplete{
			MMEUES1APID: Ptr(MMEUES1APID(1)),
			ENBUES1APID: Ptr(ENBUES1APID(7)),
			UserLocationInformation: &UserLocationInformation{
				EUTRANCGI: EUTRANCGI{PLMNIdentity: plmn, CellID: 0x0abcde1},
				TAI:       TAI{PLMNIdentity: plmn, TAC: 9},
			},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so := pdu.(*SuccessfulOutcome)

		out, err := ParseUEContextReleaseComplete(so.Value)
		if err != nil || out.UserLocationInformation == nil {
			t.Fatalf("got %+v err %v", out, err)
		}

		if out.UserLocationInformation.EUTRANCGI.CellID != 0x0abcde1 || out.UserLocationInformation.TAI.TAC != 9 {
			t.Fatalf("ULI mismatch: %+v", out.UserLocationInformation)
		}
	})

	t.Run("Request", func(t *testing.T) {
		in := &UEContextReleaseRequest{MMEUES1APID: 1, ENBUES1APID: 7, Cause: &cause}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcUEContextReleaseRequest {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextReleaseRequest(im.Value)
		if err != nil || out.MMEUES1APID != 1 || out.ENBUES1APID != 7 || deref(out.Cause) != cause {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

// UE-S1AP-IDs is extensible, so an unmodeled alternative arrives as the
// extension bit rather than NGAP's choice-Extensions container. It is still
// §10.3.1 case 6, handled on the IE's criticality: reject, so no message is
// delivered.
func TestUEContextReleaseCommandExtensionAlternative(t *testing.T) {
	w := per.NewWriter()
	w.WriteBit(true) // extension alternative

	if err := per.EncodeNormallySmall(w, per.Aligned, 0); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x00}); err != nil {
		t.Fatal(err)
	}

	value := container(t, ieField{id: IDUES1APIDs, crit: CriticalityReject, raw: perBytes(w)})

	_, err := ParseUEContextReleaseCommand(value)
	if err == nil {
		t.Fatal("parse succeeded, want a rejection")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorReject {
		t.Errorf("cause = %s, want abstract-syntax-error-reject", ase.Cause)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != IDUES1APIDs ||
		ase.IEs[0].TypeOfError != TypeOfErrorNotUnderstood {
		t.Errorf("diagnostics = %+v, want one not-understood entry for UE-S1AP-IDs", ase.IEs)
	}
}
