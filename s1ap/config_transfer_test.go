// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

// sonTransfer builds a valid leading Target eNB-ID (TS 36.413 §9.2.3.26)
// followed by opaque bytes.
func sonTransfer(t *testing.T, target TargeteNBID, opaque []byte) SONConfigurationTransfer {
	t.Helper()

	w := per.NewWriter()

	w.WriteBit(false)
	w.WriteBit(false)

	if err := target.MarshalPER(w, per.Aligned); err != nil {
		t.Fatalf("encode Target eNB-ID: %v", err)
	}

	return SONConfigurationTransfer(append(perBytes(w), opaque...))
}

func enbConfigTransferWire(t *testing.T, son SONConfigurationTransfer) []byte {
	t.Helper()

	w := per.NewWriter()

	w.WriteBit(false)

	if err := encodeIEContainer(w, per.Aligned, []ieField{{id: idSONConfigurationTransferECT, crit: CriticalityIgnore, val: son}}); err != nil {
		t.Fatalf("encode IE container: %v", err)
	}

	return perBytes(w)
}

func TestENBConfigurationTransfer_RelayRoundTrip(t *testing.T) {
	target := TargeteNBID{
		GlobalENBID: GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{Kind: ENBIDMacro, Value: 0x00abc}},
		SelectedTAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
	}
	opaque := []byte{0xde, 0xad, 0xbe, 0xef}
	son := sonTransfer(t, target, opaque)

	// Decode the ENB CONFIGURATION TRANSFER as the MME receives it.
	msg, err := ParseENBConfigurationTransfer(enbConfigTransferWire(t, son))
	if err != nil {
		t.Fatalf("ParseENBConfigurationTransfer: %v", err)
	}

	if msg.SONConfigurationTransfer == nil {
		t.Fatal("SON Configuration Transfer IE missing")
	}

	if !bytes.Equal(msg.SONConfigurationTransfer, son) {
		t.Fatalf("SON value not preserved: got %x want %x", msg.SONConfigurationTransfer, son)
	}

	// Routing: the leading Target eNB-ID must decode to the destination eNB.
	got, err := msg.SONConfigurationTransfer.TargetENBID()
	if err != nil {
		t.Fatalf("TargetENBID: %v", err)
	}

	if got.GlobalENBID != target.GlobalENBID || got.SelectedTAI != target.SelectedTAI {
		t.Fatalf("Target eNB-ID mismatch: got %+v want %+v", got, target)
	}

	// Relay: the same IE re-emitted as MME CONFIGURATION TRANSFER (proc 41),
	// carried verbatim under id-SONConfigurationTransferMCT.
	wire, err := (&MMEConfigurationTransfer{SONConfigurationTransfer: msg.SONConfigurationTransfer}).Marshal()
	if err != nil {
		t.Fatalf("MMEConfigurationTransfer.Marshal: %v", err)
	}

	pdu, err := Unmarshal(wire)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcMMEConfigurationTransfer {
		t.Fatalf("expected InitiatingMessage/ProcMMEConfigurationTransfer, got %T proc %d", pdu, pdu.procedureCode())
	}

	relayed := relayedSON(t, im.Value)
	if !bytes.Equal(relayed, son) {
		t.Fatalf("relayed SON not verbatim: got %x want %x", relayed, son)
	}
}

// The id-SONConfigurationTransferMCT value out of an MME CONFIGURATION
// TRANSFER body.
func relayedSON(t *testing.T, value []byte) []byte {
	t.Helper()

	r := per.NewReader(value)

	if _, err := r.ReadBit(); err != nil {
		t.Fatalf("body preamble: %v", err)
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatalf("decode container: %v", err)
	}

	for _, f := range fields {
		if f.id == idSONConfigurationTransferMCT {
			return f.value
		}
	}

	t.Fatal("id-SONConfigurationTransferMCT not found in MME CONFIGURATION TRANSFER")

	return nil
}

func TestENBConfigurationTransfer_NoSONIE(t *testing.T) {
	w := per.NewWriter()

	w.WriteBit(false)

	if err := encodeIEContainer(w, per.Aligned, nil); err != nil {
		t.Fatalf("encode empty container: %v", err)
	}

	msg, err := ParseENBConfigurationTransfer(perBytes(w))
	if err != nil {
		t.Fatalf("ParseENBConfigurationTransfer: %v", err)
	}

	if msg.SONConfigurationTransfer != nil {
		t.Fatalf("expected nil SON Configuration Transfer, got %x", msg.SONConfigurationTransfer)
	}
}

// §8.15.2/§8.16.2 relay the transfer verbatim, so the payload must survive
// untouched in both directions.
func TestConfigurationTransferRelaysVerbatimBothDirections(t *testing.T) {
	transfer := SONConfigurationTransfer{0x01, 0x02, 0x03, 0x04}

	t.Run("eNB to MME", func(t *testing.T) {
		b, err := (&ENBConfigurationTransfer{SONConfigurationTransfer: transfer}).Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcENBConfigurationTransfer {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseENBConfigurationTransfer(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(out.SONConfigurationTransfer, transfer) {
			t.Errorf("transfer = %x, want %x", out.SONConfigurationTransfer, transfer)
		}
	})

	t.Run("MME to eNB", func(t *testing.T) {
		b, err := (&MMEConfigurationTransfer{SONConfigurationTransfer: transfer}).Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcMMEConfigurationTransfer {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseMMEConfigurationTransfer(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(out.SONConfigurationTransfer, transfer) {
			t.Errorf("transfer = %x, want %x", out.SONConfigurationTransfer, transfer)
		}
	})
}
