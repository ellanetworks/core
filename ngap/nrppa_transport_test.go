// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"testing"
)

const (
	goldenDownlinkUEAssociatedNRPPaTransport = "0008401e000004000a0002000100550002000200590002010a002e000504deadbeef"
	goldenUplinkUEAssociatedNRPPaTransport   = "0032401e000004000a0002000100550002000200590002010a002e000504deadbeef"
)

func TestDownlinkUEAssociatedNRPPaTransportGolden(t *testing.T) {
	in := DownlinkUEAssociatedNRPPaTransport{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		RoutingID: RoutingID{0x0a},
		NRPPaPDU:  NRPPaPDU{0xde, 0xad, 0xbe, 0xef},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenDownlinkUEAssociatedNRPPaTransport {
		t.Fatalf("encoded %s, want %s", got, goldenDownlinkUEAssociatedNRPPaTransport)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcDownlinkUEAssociatedNRPPaTransport {
		t.Fatalf("decoded %T, want an initiating DownlinkUEAssociatedNRPPaTransport", pdu)
	}

	out, err := ParseDownlinkUEAssociatedNRPPaTransport(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 ||
		!bytes.Equal(out.RoutingID, in.RoutingID) || !bytes.Equal(out.NRPPaPDU, in.NRPPaPDU) {
		t.Fatalf("round trip %+v", out)
	}
}

func TestUplinkUEAssociatedNRPPaTransportGolden(t *testing.T) {
	in := UplinkUEAssociatedNRPPaTransport{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		RoutingID: RoutingID{0x0a},
		NRPPaPDU:  NRPPaPDU{0xde, 0xad, 0xbe, 0xef},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenUplinkUEAssociatedNRPPaTransport {
		t.Fatalf("encoded %s, want %s", got, goldenUplinkUEAssociatedNRPPaTransport)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcUplinkUEAssociatedNRPPaTransport {
		t.Fatalf("decoded %T, want an initiating UplinkUEAssociatedNRPPaTransport", pdu)
	}

	out, err := ParseUplinkUEAssociatedNRPPaTransport(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out.RoutingID, in.RoutingID) || !bytes.Equal(out.NRPPaPDU, in.NRPPaPDU) {
		t.Fatalf("round trip %+v", out)
	}
}

// TS 38.413 §9.3.3.14 makes RoutingID an OCTET STRING; TS 36.413 §9.2.3.31
// makes its Routing-ID an INTEGER (0..255). The two are different shapes, so a
// multi-octet routing id must survive here even though S1AP could not carry it.
func TestRoutingIDCarriesMultipleOctets(t *testing.T) {
	in := UplinkUEAssociatedNRPPaTransport{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		RoutingID: RoutingID{0x01, 0x02, 0x03, 0x04},
		NRPPaPDU:  NRPPaPDU{0x00},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseUplinkUEAssociatedNRPPaTransport(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out.RoutingID, in.RoutingID) {
		t.Fatalf("routing id = %x, want %x", out.RoutingID, in.RoutingID)
	}
}

// Every IE is mandatory and reject criticality, so the sender is bound by
// §9.1.1 even where §10.3.5 would let a receiver carry on.
func TestNRPPaTransportRefusesMissingMandatoryIEs(t *testing.T) {
	for _, tt := range []struct {
		name string
		omit func(*UplinkUEAssociatedNRPPaTransport)
	}{
		{"RoutingID", func(m *UplinkUEAssociatedNRPPaTransport) { m.RoutingID = nil }},
		{"NRPPaPDU", func(m *UplinkUEAssociatedNRPPaTransport) { m.NRPPaPDU = nil }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := UplinkUEAssociatedNRPPaTransport{
				AMFUENGAPID: 1, RANUENGAPID: 2,
				RoutingID: RoutingID{0x0a}, NRPPaPDU: NRPPaPDU{0x00},
			}
			tt.omit(&m)

			if _, err := m.Marshal(); err == nil {
				t.Errorf("encoded an NRPPa transport with no %s", tt.name)
			}
		})
	}
}
