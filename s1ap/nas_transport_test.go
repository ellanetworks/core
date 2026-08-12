// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

// A real INITIAL UE MESSAGE carrying an LTE Attach Request.
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
		deref(msg2.EUTRANCGI) != deref(msg.EUTRANCGI) ||
		deref(msg2.RRCEstablishmentCause) != deref(msg.RRCEstablishmentCause) ||
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
			ENBUES1APID: 1, NASPDU: nas, TAI: tai, EUTRANCGI: &cgi,
			RRCEstablishmentCause: Ptr(RRCCauseMOSignalling),
		}

		out, err := roundTripInitialUE(t, in)
		if err != nil {
			t.Fatal(err)
		}

		if out.ENBUES1APID != in.ENBUES1APID || out.TAI != in.TAI || deref(out.EUTRANCGI) != deref(in.EUTRANCGI) ||
			deref(out.RRCEstablishmentCause) != deref(in.RRCEstablishmentCause) || !bytes.Equal(out.NASPDU, in.NASPDU) {
			t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
		}

		if out.STMSI != nil {
			t.Fatalf("S-TMSI = %+v, want nil when absent", out.STMSI)
		}
	})

	t.Run("InitialUEMessageWithSTMSI", func(t *testing.T) {
		in := &InitialUEMessage{
			ENBUES1APID: 9, NASPDU: NASPDU{0xc7, 0x00, 0x12, 0x34}, TAI: tai, EUTRANCGI: &cgi,
			RRCEstablishmentCause: Ptr(RRCCauseMOSignalling),
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
			ENBUES1APID: 3, NASPDU: nas, TAI: tai, EUTRANCGI: &cgi,
			RRCEstablishmentCause: Ptr(RRCCauseMOSignalling),
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
		in := &UplinkNASTransport{MMEUES1APID: 42, ENBUES1APID: 1, NASPDU: nas, EUTRANCGI: &cgi, TAI: &tai}

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
			deref(out.TAI) != deref(in.TAI) || deref(out.EUTRANCGI) != deref(in.EUTRANCGI) ||
			!bytes.Equal(out.NASPDU, in.NASPDU) {
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

const (
	goldenUplinkNASTransport        = "000d402c000005000000020001000800020001001a0003020741006440080000f1100abcde10004340060000f1100001"
	goldenUplinkNASTransportWideIDs = "000d403400000500000005c0ffffffff0008000480ffffff001a0006050741010bf6006440080000f110fffffff0004340060000f110ffff"
)

func TestUplinkNASTransportGolden(t *testing.T) {
	tests := []struct {
		name string
		msg  *UplinkNASTransport
		want string
	}{
		{
			"minimal",
			&UplinkNASTransport{
				MMEUES1APID: 1, ENBUES1APID: 1, NASPDU: NASPDU{0x07, 0x41},
				EUTRANCGI: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1},
				TAI:       &TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
			},
			goldenUplinkNASTransport,
		},
		{
			// The widest ids either field can carry: MME-UE-S1AP-ID is 32 bits,
			// eNB-UE-S1AP-ID 24, and the cell identity 28.
			"wide ids",
			&UplinkNASTransport{
				MMEUES1APID: 0xffffffff, ENBUES1APID: 0xffffff, NASPDU: NASPDU{0x07, 0x41, 0x01, 0x0b, 0xf6},
				EUTRANCGI: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0xfffffff},
				TAI:       &TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 0xffff},
			},
			goldenUplinkNASTransportWideIDs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.msg.Marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
			}

			pdu, err := Unmarshal(mustHex(t, tt.want))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			out, err := ParseUplinkNASTransport(pdu.value())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if out.MMEUES1APID != tt.msg.MMEUES1APID || out.ENBUES1APID != tt.msg.ENBUES1APID ||
				deref(out.EUTRANCGI) != deref(tt.msg.EUTRANCGI) || deref(out.TAI) != deref(tt.msg.TAI) ||
				!bytes.Equal(out.NASPDU, tt.msg.NASPDU) {
				t.Fatalf("decode mismatch:\n  got  %+v\n  want %+v", out, tt.msg)
			}
		})
	}
}

// The three UE-addressing IEs are mandatory-reject, so §10.3.5 stops the
// message; E-UTRAN CGI and TAI are mandatory-ignore, so their absence is
// reported and the message still delivered.
func TestUplinkNASTransportMissingIEs(t *testing.T) {
	if _, err := (&UplinkNASTransport{}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error")
	}

	if _, err := ParseUplinkNASTransport(container(t,
		ieField{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(42)},
		ieField{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(7)},
	)); err == nil {
		t.Error("decoded a message with no NAS-PDU, want it rejected")
	}

	msg, err := ParseUplinkNASTransport(container(t,
		ieField{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(42)},
		ieField{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(7)},
		ieField{id: IDNASPDU, crit: CriticalityReject, val: NASPDU{0x07}},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.EUTRANCGI != nil || msg.TAI != nil {
		t.Errorf("absent IEs decoded to non-nil: %+v", msg)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if ie.TypeOfError == TypeOfErrorMissing && (ie.ID == IDEUTRANCGI || ie.ID == IDTAI) {
			missing++
		}
	}

	if missing != 2 {
		t.Errorf("reported %d missing IEs, want 2: %+v", missing, msg.Diagnostics().IEs)
	}
}

const (
	goldenDownlinkNASTransport        = "000b4016000003000000020001000800020001001a0003020742"
	goldenDownlinkNASTransportWideIDs = "000b401e00000300000005c0ffffffff0008000480ffffff001a0006050742010203"
)

func TestDownlinkNASTransportGolden(t *testing.T) {
	tests := []struct {
		name string
		msg  *DownlinkNASTransport
		want string
	}{
		{
			"minimal",
			&DownlinkNASTransport{MMEUES1APID: 1, ENBUES1APID: 1, NASPDU: NASPDU{0x07, 0x42}},
			goldenDownlinkNASTransport,
		},
		{
			// The widest ids either field can carry: MME-UE-S1AP-ID is 32 bits,
			// eNB-UE-S1AP-ID 24.
			"wide ids",
			&DownlinkNASTransport{
				MMEUES1APID: 0xffffffff, ENBUES1APID: 0xffffff,
				NASPDU: NASPDU{0x07, 0x42, 0x01, 0x02, 0x03},
			},
			goldenDownlinkNASTransportWideIDs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.msg.Marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
			}

			pdu, err := Unmarshal(mustHex(t, tt.want))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			out, err := ParseDownlinkNASTransport(pdu.value())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if out.MMEUES1APID != tt.msg.MMEUES1APID || out.ENBUES1APID != tt.msg.ENBUES1APID ||
				!bytes.Equal(out.NASPDU, tt.msg.NASPDU) {
				t.Fatalf("decode mismatch:\n  got  %+v\n  want %+v", out, tt.msg)
			}
		})
	}
}

// Every modeled IE is mandatory-reject, so §9.1.2.1 refuses to encode any unset
// one and §10.3.5 stops an arriving message that omits one.
func TestDownlinkNASTransportMissingIEs(t *testing.T) {
	if _, err := (&DownlinkNASTransport{}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error")
	}

	if _, err := (&DownlinkNASTransport{MMEUES1APID: 1, ENBUES1APID: 1}).Marshal(); err == nil {
		t.Error("Marshal() with no NAS-PDU = nil error, want a required-IE error")
	}

	if _, err := ParseDownlinkNASTransport(container(t,
		ieField{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(42)},
		ieField{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(7)},
	)); err == nil {
		t.Error("decoded a message with no NAS-PDU, want it rejected")
	}
}
