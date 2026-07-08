// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func TestNASNonDeliveryIndicationRoundTrip(t *testing.T) {
	in := &NASNonDeliveryIndication{
		MMEUES1APID: 42,
		ENBUES1APID: 1,
		NASPDU:      NASPDU{0x7E, 0x00, 0x42},
		Cause:       Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnknownMMEUES1APID},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Initiating message, procedureCode 16 (TS 36.413 §9.1.7.4).
	if b[1] != 0x10 {
		t.Fatalf("procedureCode byte = %#x, want 0x10", b[1])
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseNASNonDeliveryIndication(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID ||
		out.Cause != in.Cause || !bytes.Equal(out.NASPDU, in.NASPDU) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}
