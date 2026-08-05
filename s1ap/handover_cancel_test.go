// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"
)

func TestHandoverCancelRoundTrip(t *testing.T) {
	in := &HandoverCancel{MMEUES1APID: 7, ENBUES1APID: 2, Cause: Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 5})}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverCancel {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverCancel(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || deref(out.Cause) != deref(in.Cause) {
		t.Fatalf("cancel = %+v, want %+v", out, in)
	}

	ack := &HandoverCancelAcknowledge{MMEUES1APID: Ptr(MMEUES1APID(7)), ENBUES1APID: Ptr(ENBUES1APID(2))}

	ab, err := ack.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	apdu, err := Unmarshal(ab)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := apdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcHandoverCancel {
		t.Fatalf("got %T procedureCode %d", apdu, apdu.procedureCode())
	}

	aout, err := ParseHandoverCancelAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse ack: %v", err)
	}

	if deref(aout.MMEUES1APID) != deref(ack.MMEUES1APID) || deref(aout.ENBUES1APID) != deref(ack.ENBUES1APID) {
		t.Fatalf("cancel ack = %+v", aout)
	}
}
