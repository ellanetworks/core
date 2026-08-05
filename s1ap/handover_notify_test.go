// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"
)

func TestHandoverNotifyRoundTrip(t *testing.T) {
	in := &HandoverNotify{
		MMEUES1APID: 7,
		ENBUES1APID: 9,
		EUTRANCGI:   Ptr(EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x123c601}),
		TAI:         Ptr(TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7}),
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
	if !ok || im.ProcedureCode != ProcHandoverNotification {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverNotify(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID ||
		deref(out.EUTRANCGI) != deref(in.EUTRANCGI) || deref(out.TAI) != deref(in.TAI) {
		t.Fatalf("notify = %+v, want %+v", out, in)
	}
}
