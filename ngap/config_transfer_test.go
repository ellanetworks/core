// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"
)

// Golden RAN CONFIGURATION TRANSFER PDUs
const (
	goldenUplinkRANConfigTransfer        = "0030402700000100634020000000f110100001020000f1100000010000f11010000a0b0000f11000000200"
	goldenDownlinkRANConfigTransfer      = "0006402700000100624020000000f110100001020000f1100000010000f11010000a0b0000f11000000200"
	goldenDownlinkRANConfigTransferEmpty = "00064003000000"
)

// sonTransferBytes is the SON Configuration Transfer open-type payload out of
// the golden PDUs, which is exactly what the AMF relays verbatim.
func sonTransferBytes(t *testing.T, golden string) SONConfigurationTransfer {
	t.Helper()

	pdu, err := Unmarshal(mustHex(t, golden))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok {
		t.Fatalf("got %T, want an initiating message", pdu)
	}

	msg, err := ParseUplinkRANConfigurationTransfer(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	return msg.SONConfigurationTransfer
}

func TestRANConfigurationTransferGoldenDecode(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		proc ProcedureCode
	}{
		{"uplink", goldenUplinkRANConfigTransfer, ProcUplinkRANConfigurationTransfer},
		{"downlink", goldenDownlinkRANConfigTransfer, ProcDownlinkRANConfigurationTransfer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdu, err := Unmarshal(mustHex(t, tt.hex))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			im, ok := pdu.(*InitiatingMessage)
			if !ok || im.ProcedureCode != tt.proc {
				t.Fatalf("got %T procedureCode %d, want %d", pdu, pdu.procedureCode(), tt.proc)
			}

			var transfer SONConfigurationTransfer

			if tt.proc == ProcUplinkRANConfigurationTransfer {
				msg, err := ParseUplinkRANConfigurationTransfer(im.Value)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}

				transfer = msg.SONConfigurationTransfer
			} else {
				msg, err := ParseDownlinkRANConfigurationTransfer(im.Value)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}

				transfer = msg.SONConfigurationTransfer
			}

			if len(transfer) == 0 {
				t.Fatal("SON Configuration Transfer is empty")
			}
		})
	}
}

// TS 38.413 §8.8.1.2 has the AMF relay the transfer "transparently", so what it
// forwards must be byte-identical to what arrived. Only the leading Target RAN
// Node ID is read, for routing.
func TestSONConfigurationTransferRelaysVerbatim(t *testing.T) {
	transfer := sonTransferBytes(t, goldenUplinkRANConfigTransfer)

	out, err := (&DownlinkRANConfigurationTransfer{SONConfigurationTransfer: transfer}).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if want := mustHex(t, goldenDownlinkRANConfigTransfer); !bytes.Equal(out, want) {
		t.Fatalf("relayed downlink mismatch:\n  got  %x\n  want %x", out, want)
	}
}

// The AMF routes on the Target RAN Node ID at the head of the transfer, so that
// field must decode without touching the rest (TS 38.413 §9.3.3.14).
func TestSONConfigurationTransferTargetRANNodeID(t *testing.T) {
	transfer := sonTransferBytes(t, goldenUplinkRANConfigTransfer)

	target, err := transfer.TargetRANNodeID()
	if err != nil {
		t.Fatalf("TargetRANNodeID: %v", err)
	}

	want := GlobalRANNodeID{
		Kind:         RANNodeIDGNB,
		PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
		Value:        0x000102,
		Bits:         24,
	}

	if target.GlobalRANNodeID != want {
		t.Errorf("GlobalRANNodeID = %+v, want %+v", target.GlobalRANNodeID, want)
	}

	if target.SelectedTAI.TAC != 1 {
		t.Errorf("SelectedTAI.TAC = %d, want 1", target.SelectedTAI.TAC)
	}

	if target.SelectedTAI.PLMNIdentity != (PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Errorf("SelectedTAI.PLMNIdentity = %v", target.SelectedTAI.PLMNIdentity)
	}
}

// A truncated transfer must report an error rather than route on a zero target.
func TestSONConfigurationTransferTargetRANNodeIDRejectsTruncated(t *testing.T) {
	for _, n := range []int{0, 1, 4} {
		transfer := sonTransferBytes(t, goldenUplinkRANConfigTransfer)[:n]

		if got, err := transfer.TargetRANNodeID(); err == nil {
			t.Errorf("%d octets decoded to %+v, want an error", n, got)
		}
	}
}

// The IE is optional, so a transfer carrying nothing is a valid message.
func TestDownlinkRANConfigurationTransferEmpty(t *testing.T) {
	got, err := (&DownlinkRANConfigurationTransfer{}).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if want := mustHex(t, goldenDownlinkRANConfigTransferEmpty); !bytes.Equal(got, want) {
		t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
	}

	pdu, err := Unmarshal(got)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseDownlinkRANConfigurationTransfer(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.SONConfigurationTransfer != nil {
		t.Errorf("SONConfigurationTransfer = %x, want nil", msg.SONConfigurationTransfer)
	}
}
