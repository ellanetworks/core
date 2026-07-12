// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/s1ap/aper"
)

// Golden vectors are the aligned-PER encoding of the full S1AP-PDU produced by
// pycrate from the 3GPP TS 36.413 S1AP ASN.1, not by this codec.
const (
	goldenDownlinkLPPaTransport = "002c401f00000400000003401092000800020007009400010300930006050005abcdef"
	goldenUplinkLPPaTransport   = "002d401f0000040000000200010008000480ffffff00940001ff00930005046000f110"
)

func TestGoldenDownlinkLPPaTransport(t *testing.T) {
	in := &DownlinkUEAssociatedLPPaTransport{
		MMEUES1APID: 4242,
		ENBUES1APID: 7,
		RoutingID:   3,
		LPPaPDU:     []byte{0x00, 0x05, 0xab, 0xcd, 0xef},
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if got := hex.EncodeToString(wire); got != goldenDownlinkLPPaTransport {
		t.Fatalf("downlink\n got=%s\nwant=%s", got, goldenDownlinkLPPaTransport)
	}
}

func TestGoldenUplinkLPPaTransport(t *testing.T) {
	in := &UplinkUEAssociatedLPPaTransport{
		MMEUES1APID: 1,
		ENBUES1APID: 16777215,
		RoutingID:   255,
		LPPaPDU:     []byte{0x60, 0x00, 0xf1, 0x10},
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if got := hex.EncodeToString(wire); got != goldenUplinkLPPaTransport {
		t.Fatalf("uplink\n got=%s\nwant=%s", got, goldenUplinkLPPaTransport)
	}
}

func TestDownlinkUEAssociatedLPPaTransportRoundTrip(t *testing.T) {
	lppaPDU := []byte{0x00, 0x05, 0xab, 0xcd, 0xef}

	in := &DownlinkUEAssociatedLPPaTransport{
		MMEUES1APID: 4242,
		ENBUES1APID: 7,
		RoutingID:   3,
		LPPaPDU:     lppaPDU,
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(wire)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcDownlinkUEAssociatedLPPaTransport {
		t.Fatalf("pdu = %T proc = %v, want DownlinkUEAssociatedLPPaTransport (44)", pdu, im.ProcedureCode)
	}

	out, err := ParseDownlinkUEAssociatedLPPaTransport(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.RoutingID != in.RoutingID {
		t.Fatalf("ids: mme=%d enb=%d routing=%d", out.MMEUES1APID, out.ENBUES1APID, out.RoutingID)
	}

	if !bytes.Equal(out.LPPaPDU, lppaPDU) {
		t.Fatalf("lppa pdu = %x, want %x", out.LPPaPDU, lppaPDU)
	}
}

func TestUplinkUEAssociatedLPPaTransportRoundTrip(t *testing.T) {
	lppaPDU := []byte{0x60, 0x00, 0xf1, 0x10}

	in := &UplinkUEAssociatedLPPaTransport{
		MMEUES1APID: 1,
		ENBUES1APID: 16777215,
		RoutingID:   255,
		LPPaPDU:     lppaPDU,
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(wire)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcUplinkUEAssociatedLPPaTransport {
		t.Fatalf("pdu = %T proc = %v, want UplinkUEAssociatedLPPaTransport (45)", pdu, im.ProcedureCode)
	}

	out, err := ParseUplinkUEAssociatedLPPaTransport(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.RoutingID != in.RoutingID {
		t.Fatalf("ids: mme=%d enb=%d routing=%d", out.MMEUES1APID, out.ENBUES1APID, out.RoutingID)
	}

	if !bytes.Equal(out.LPPaPDU, lppaPDU) {
		t.Fatalf("lppa pdu = %x, want %x", out.LPPaPDU, lppaPDU)
	}
}

func TestLPPaTransportEmptyPDU(t *testing.T) {
	in := &DownlinkUEAssociatedLPPaTransport{MMEUES1APID: 1, ENBUES1APID: 1, RoutingID: 0, LPPaPDU: nil}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, _ := Unmarshal(wire)

	out, err := ParseDownlinkUEAssociatedLPPaTransport(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(out.LPPaPDU) != 0 {
		t.Fatalf("lppa pdu = %x, want empty", out.LPPaPDU)
	}
}

// TestLPPaTransportUnknownIE decodes a message carrying an IE the type does not
// model; it must round-trip (be preserved) rather than fail.
func TestLPPaTransportUnknownIE(t *testing.T) {
	var w aper.Writer

	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: MMEUES1APID(9).encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: ENBUES1APID(9).encode},
		{id: idRoutingID, crit: CriticalityReject, enc: RoutingID(0).encode},
		{id: idLPPaPDU, crit: CriticalityReject, enc: LPPaPDU{0x01}.encode},
		{id: 999, crit: CriticalityIgnore, enc: func(w *aper.Writer) error { w.WriteOctets([]byte{0xaa}); return nil }},
	}

	if err := encodeIEContainer(&w, fields); err != nil {
		t.Fatal(err)
	}

	out, err := ParseUplinkUEAssociatedLPPaTransport(w.Bytes())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != 9 || len(out.UnknownIEs()) != 1 || out.UnknownIEs()[0].ID != 999 {
		t.Fatalf("unknown IE not preserved: %+v", out.UnknownIEs())
	}
}

func TestLPPaTransportMissingMandatoryIE(t *testing.T) {
	var w aper.Writer

	w.WriteSequencePreamble(true, false, nil)

	// MME-UE-S1AP-ID and eNB-UE-S1AP-ID only; Routing-ID and LPPa-PDU omitted.
	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: MMEUES1APID(1).encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: ENBUES1APID(1).encode},
	}

	if err := encodeIEContainer(&w, fields); err != nil {
		t.Fatal(err)
	}

	if _, err := ParseDownlinkUEAssociatedLPPaTransport(w.Bytes()); err == nil {
		t.Fatal("expected missing-mandatory-IE error")
	}
}
