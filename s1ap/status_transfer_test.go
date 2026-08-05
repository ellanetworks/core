// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func TestStatusTransferRelayRoundTrip(t *testing.T) {
	container := StatusTransferContainer{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02}

	enb := &ENBStatusTransfer{MMEUES1APID: 7, ENBUES1APID: 2, Container: container}

	b, err := enb.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcENBStatusTransfer {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	parsed, err := ParseENBStatusTransfer(im.Value)
	if err != nil {
		t.Fatalf("parse enb: %v", err)
	}

	if !bytes.Equal(parsed.Container, container) {
		t.Fatalf("relayed container = %x, want %x", parsed.Container, container)
	}

	// Relay into an MME STATUS TRANSFER addressed to the target eNB, then decode
	// the container back out unchanged.
	mme := &MMEStatusTransfer{MMEUES1APID: parsed.MMEUES1APID, ENBUES1APID: 9, Container: parsed.Container}

	mb, err := mme.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	mpdu, err := Unmarshal(mb)
	if err != nil {
		t.Fatal(err)
	}

	mim, ok := mpdu.(*InitiatingMessage)
	if !ok || mim.ProcedureCode != ProcMMEStatusTransfer {
		t.Fatalf("got %T procedureCode %d", mpdu, mpdu.procedureCode())
	}

	mout, err := ParseMMEStatusTransfer(mim.Value)
	if err != nil {
		t.Fatalf("parse mme: %v", err)
	}

	if mout.ENBUES1APID != 9 || !bytes.Equal(mout.Container, container) {
		t.Fatalf("mme status transfer = %+v", mout)
	}
}
