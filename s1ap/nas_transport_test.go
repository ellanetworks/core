// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

// goldenInitialUEMessage is a real INITIAL UE MESSAGE carrying an LTE Attach
// Request
const goldenInitialUEMessage = "000c406f000006000800020001001a003c3b17df675aa8050741020bf600f110000201030003e605f070000010000502" +
	"15d011d15200f11030395c0a003103e5e0349011035758a65d0100e0c1004300060000f1103039006440080000f1108c" +
	"3378200086400130004b00070000f110000201"

func TestInitialUEMessageGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenInitialUEMessage))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcInitialUEMessage {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	msg, err := ParseInitialUEMessage(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.ENBUES1APID != 1 {
		t.Fatalf("eNB-UE-S1AP-ID = %d, want 1", msg.ENBUES1APID)
	}

	if len(msg.NASPDU) == 0 {
		t.Fatal("NAS-PDU is empty")
	}

	// Semantic round-trip: re-encoding (including preserved unknown IEs) and
	// re-decoding must reproduce the modeled fields.
	b2, err := msg.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu2, err := Unmarshal(b2)
	if err != nil {
		t.Fatal(err)
	}

	msg2, err := ParseInitialUEMessage(pdu2.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if msg2.ENBUES1APID != msg.ENBUES1APID || msg2.TAI != msg.TAI ||
		msg2.EUTRANCGI != msg.EUTRANCGI || msg2.RRCEstablishmentCause != msg.RRCEstablishmentCause ||
		!bytes.Equal(msg2.NASPDU, msg.NASPDU) {
		t.Fatalf("semantic round-trip mismatch:\n  %+v\n  %+v", msg, msg2)
	}
}

func TestNASTransportRoundTrips(t *testing.T) {
	tai := TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 0x3039}
	cgi := EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}
	nas := NASPDU{0x07, 0x41, 0x01, 0x0b, 0xf6}

	t.Run("InitialUEMessage", func(t *testing.T) {
		in := &InitialUEMessage{
			ENBUES1APID: 1, NASPDU: nas, TAI: tai, EUTRANCGI: cgi,
			RRCEstablishmentCause: RRCCauseMOSignalling,
		}

		out, err := roundTripInitialUE(t, in)
		if err != nil {
			t.Fatal(err)
		}

		if out.ENBUES1APID != in.ENBUES1APID || out.TAI != in.TAI || out.EUTRANCGI != in.EUTRANCGI ||
			out.RRCEstablishmentCause != in.RRCEstablishmentCause || !bytes.Equal(out.NASPDU, in.NASPDU) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}

		if out.STMSI != nil {
			t.Fatalf("S-TMSI = %+v, want nil when absent", out.STMSI)
		}
	})

	t.Run("InitialUEMessageWithSTMSI", func(t *testing.T) {
		in := &InitialUEMessage{
			ENBUES1APID: 9, NASPDU: NASPDU{0xc7, 0x00, 0x12, 0x34}, TAI: tai, EUTRANCGI: cgi,
			RRCEstablishmentCause: RRCCauseMOSignalling,
			STMSI:                 &STMSI{MMEC: 0x07, MTMSI: 0xdeadbeef},
		}

		out, err := roundTripInitialUE(t, in)
		if err != nil {
			t.Fatal(err)
		}

		if out.STMSI == nil || *out.STMSI != *in.STMSI {
			t.Fatalf("S-TMSI round-trip mismatch:\n  in  %+v\n  out %+v", in.STMSI, out.STMSI)
		}

		if out.ENBUES1APID != in.ENBUES1APID || out.TAI != in.TAI || !bytes.Equal(out.NASPDU, in.NASPDU) {
			t.Fatalf("field mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})

	t.Run("InitialUEMessageWithGUMMEI", func(t *testing.T) {
		in := &InitialUEMessage{
			ENBUES1APID: 3, NASPDU: nas, TAI: tai, EUTRANCGI: cgi,
			RRCEstablishmentCause: RRCCauseMOSignalling,
			GUMMEI:                &GUMMEI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, MMEGroupID: MMEGroupID{0x00, 0x01}, MMECode: 0x07},
		}

		out, err := roundTripInitialUE(t, in)
		if err != nil {
			t.Fatal(err)
		}

		if out.GUMMEI == nil || *out.GUMMEI != *in.GUMMEI {
			t.Fatalf("GUMMEI round-trip mismatch:\n  in  %+v\n  out %+v", in.GUMMEI, out.GUMMEI)
		}
	})

	t.Run("UplinkNASTransport", func(t *testing.T) {
		in := &UplinkNASTransport{MMEUES1APID: 42, ENBUES1APID: 1, NASPDU: nas, EUTRANCGI: cgi, TAI: tai}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, _ := Unmarshal(b)

		out, err := ParseUplinkNASTransport(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID ||
			out.TAI != in.TAI || out.EUTRANCGI != in.EUTRANCGI || !bytes.Equal(out.NASPDU, in.NASPDU) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})

	t.Run("DownlinkNASTransport", func(t *testing.T) {
		in := &DownlinkNASTransport{MMEUES1APID: 42, ENBUES1APID: 1, NASPDU: nas}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		// DownlinkNASTransport is MME-originated; an initiatingMessage with
		// procedureCode 11.
		if b[1] != 0x0b {
			t.Fatalf("procedureCode byte = %#x, want 0x0b", b[1])
		}

		pdu, _ := Unmarshal(b)

		out, err := ParseDownlinkNASTransport(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID ||
			!bytes.Equal(out.NASPDU, in.NASPDU) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}
	})
}

func roundTripInitialUE(t *testing.T, in *InitialUEMessage) (*InitialUEMessage, error) {
	t.Helper()

	b, err := in.Marshal()
	if err != nil {
		return nil, err
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		return nil, err
	}

	return ParseInitialUEMessage(pdu.(*InitiatingMessage).Value)
}
