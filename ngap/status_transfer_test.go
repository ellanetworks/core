// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"
)

func TestStatusTransferRelayRoundTrip(t *testing.T) {
	container := StatusTransferContainer{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02}

	up := &UplinkRANStatusTransfer{AMFUENGAPID: 7, RANUENGAPID: 2, Container: container}

	b, err := up.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcUplinkRANStatusTransfer {
		t.Fatalf("decoded %T, want an initiating UplinkRANStatusTransfer", pdu)
	}

	parsed, err := ParseUplinkRANStatusTransfer(im.Value)
	if err != nil {
		t.Fatalf("parse uplink: %v", err)
	}

	if !bytes.Equal(parsed.Container, container) {
		t.Fatalf("relayed container = %x, want %x", parsed.Container, container)
	}

	// Relay into a DOWNLINK RAN STATUS TRANSFER addressed to the target node, then
	// decode the container back out unchanged.
	down := &DownlinkRANStatusTransfer{AMFUENGAPID: parsed.AMFUENGAPID, RANUENGAPID: 9, Container: parsed.Container}

	db, err := down.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	dpdu, err := Unmarshal(db)
	if err != nil {
		t.Fatal(err)
	}

	dim, ok := dpdu.(*InitiatingMessage)
	if !ok || dim.ProcedureCode != ProcDownlinkRANStatusTransfer {
		t.Fatalf("decoded %T, want an initiating DownlinkRANStatusTransfer", dpdu)
	}

	dout, err := ParseDownlinkRANStatusTransfer(dim.Value)
	if err != nil {
		t.Fatalf("parse downlink: %v", err)
	}

	if dout.RANUENGAPID != 9 || !bytes.Equal(dout.Container, container) {
		t.Fatalf("downlink status transfer = %+v", dout)
	}
}
