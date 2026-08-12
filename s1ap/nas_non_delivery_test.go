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
		Cause:       Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnknownMMEUES1APID}),
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
		deref(out.Cause) != deref(in.Cause) || !bytes.Equal(out.NASPDU, in.NASPDU) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

const (
	goldenNASNonDelivery         = "0010401c000004000000020001000800020001001a4003027e00000240020000"
	goldenNASNonDeliveryWideIDs  = "0010402400000400000005c0ffffffff0008000480ffffff001a4006057e010203040002400201e0"
	goldenNASNonDeliveryNASCause = "0010401a00000400000002002a000800020007001a4002017e0002400124"
)

func TestNASNonDeliveryIndicationGolden(t *testing.T) {
	tests := []struct {
		name string
		msg  *NASNonDeliveryIndication
		want string
	}{
		{
			"minimal",
			&NASNonDeliveryIndication{
				MMEUES1APID: 1, ENBUES1APID: 1, NASPDU: NASPDU{0x7e, 0x00},
				Cause: Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}),
			},
			goldenNASNonDelivery,
		},
		{
			// The widest ids either field can carry: MME-UE-S1AP-ID is 32 bits,
			// eNB-UE-S1AP-ID 24.
			"wide ids",
			&NASNonDeliveryIndication{
				MMEUES1APID: 0xffffffff, ENBUES1APID: 0xffffff,
				NASPDU: NASPDU{0x7e, 0x01, 0x02, 0x03, 0x04},
				Cause:  Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnknownPairUES1APID}),
			},
			goldenNASNonDeliveryWideIDs,
		},
		{
			"nas cause group",
			&NASNonDeliveryIndication{
				MMEUES1APID: 42, ENBUES1APID: 7, NASPDU: NASPDU{0x7e},
				Cause: Ptr(Cause{Group: CauseGroupNAS, Value: CauseNASDetach}),
			},
			goldenNASNonDeliveryNASCause,
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

			out, err := ParseNASNonDeliveryIndication(pdu.value())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if out.MMEUES1APID != tt.msg.MMEUES1APID || out.ENBUES1APID != tt.msg.ENBUES1APID ||
				deref(out.Cause) != deref(tt.msg.Cause) || !bytes.Equal(out.NASPDU, tt.msg.NASPDU) {
				t.Fatalf("decode mismatch:\n  got  %+v\n  want %+v", out, tt.msg)
			}
		})
	}
}

// The two UE ids are mandatory-reject, so §10.3.5 stops the message; NAS-PDU and
// Cause are mandatory-ignore, so their absence is reported and delivered.
func TestNASNonDeliveryIndicationMissingIEs(t *testing.T) {
	if _, err := (&NASNonDeliveryIndication{}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error")
	}

	if _, err := ParseNASNonDeliveryIndication(container(t,
		ieField{id: IDNASPDU, crit: CriticalityIgnore, val: NASPDU{0x7e}},
		ieField{id: IDCause, crit: CriticalityIgnore, val: Cause{Group: CauseGroupNAS, Value: CauseNASDetach}},
	)); err == nil {
		t.Error("decoded a message with neither UE id, want it rejected")
	}

	msg, err := ParseNASNonDeliveryIndication(container(t,
		ieField{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(42)},
		ieField{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(7)},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.NASPDU != nil || msg.Cause != nil {
		t.Errorf("absent IEs decoded to non-nil: %+v", msg)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if ie.TypeOfError == TypeOfErrorMissing && (ie.ID == IDNASPDU || ie.ID == IDCause) {
			missing++
		}
	}

	if missing != 2 {
		t.Errorf("reported %d missing IEs, want 2: %+v", missing, msg.Diagnostics().IEs)
	}
}
