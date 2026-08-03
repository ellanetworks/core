// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"
)

// Golden NAS NON DELIVERY INDICATION PDUs from free5gc/ngap v1.1.3.
const (
	goldenNASNonDelivery         = "0013401c000004000a0002000100550002000100264003027e00000f40020000"
	goldenNASNonDeliveryWideIDs  = "00134026000004000a000680ffffffffff00550005c0ffffffff00264006057e01020304000f400203c0"
	goldenNASNonDeliveryNASCause = "0013401a000004000a0002002a00550002000700264002017e000f400148"
)

func TestNASNonDeliveryIndicationRoundTrip(t *testing.T) {
	in := &NASNonDeliveryIndication{
		AMFUENGAPID: 42,
		RANUENGAPID: 1,
		NASPDU:      NASPDU{0x7E, 0x00, 0x42},
		Cause:       Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkInconsistentRemoteUEID}),
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Initiating message, procedureCode 19 (TS 38.413 §9.2.5.4).
	if b[1] != 0x13 {
		t.Fatalf("procedureCode byte = %#x, want 0x13", b[1])
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseNASNonDeliveryIndication(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != in.AMFUENGAPID || out.RANUENGAPID != in.RANUENGAPID ||
		deref(out.Cause) != deref(in.Cause) || !bytes.Equal(out.NASPDU, in.NASPDU) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

func TestNASNonDeliveryIndicationGolden(t *testing.T) {
	tests := []struct {
		name string
		msg  *NASNonDeliveryIndication
		want string
	}{
		{
			"minimal",
			&NASNonDeliveryIndication{
				AMFUENGAPID: 1, RANUENGAPID: 1, NASPDU: NASPDU{0x7e, 0x00},
				Cause: Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}),
			},
			goldenNASNonDelivery,
		},
		{
			// The widest ids either field can carry: the AMF's is 40 bits,
			// past what a uint32 holds.
			"wide ids",
			&NASNonDeliveryIndication{
				AMFUENGAPID: 0xffffffffff, RANUENGAPID: 0xffffffff,
				NASPDU: NASPDU{0x7e, 0x01, 0x02, 0x03, 0x04},
				Cause:  Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkInconsistentRemoteUEID}),
			},
			goldenNASNonDeliveryWideIDs,
		},
		{
			"nas cause group",
			&NASNonDeliveryIndication{
				AMFUENGAPID: 42, RANUENGAPID: 7, NASPDU: NASPDU{0x7e},
				Cause: Ptr(Cause{Group: CauseGroupNAS, Value: CauseNASDeregister}),
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

			if out.AMFUENGAPID != tt.msg.AMFUENGAPID || out.RANUENGAPID != tt.msg.RANUENGAPID ||
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
		ieField{id: idNASPDU, crit: CriticalityIgnore, val: NASPDU{0x7e}},
		ieField{id: idCause, crit: CriticalityIgnore, val: Cause{Group: CauseGroupNAS, Value: CauseNASDeregister}},
	)); err == nil {
		t.Error("decoded a message with neither UE id, want it rejected")
	}

	msg, err := ParseNASNonDeliveryIndication(container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, val: AMFUENGAPID(42)},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(7)},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.NASPDU != nil || msg.Cause != nil {
		t.Errorf("absent IEs decoded to non-nil: %+v", msg)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if ie.TypeOfError == TypeOfErrorMissing && (ie.ID == idNASPDU || ie.ID == idCause) {
			missing++
		}
	}

	if missing != 2 {
		t.Errorf("reported %d missing IEs, want 2: %+v", missing, msg.Diagnostics().IEs)
	}
}
