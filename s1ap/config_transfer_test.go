// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/aper"
)

// sonTransfer builds a SON Configuration Transfer value: a valid leading
// Target eNB-ID (TS 36.413 §9.2.3.26) followed by opaque bytes standing in for
// the source eNB-ID and SON Information, which the MME relays without decoding.
func sonTransfer(t *testing.T, target TargeteNBID, opaque []byte) SONConfigurationTransfer {
	t.Helper()

	var w aper.Writer

	w.WriteSequencePreamble(true, false, []bool{false})

	if err := target.encode(&w); err != nil {
		t.Fatalf("encode Target eNB-ID: %v", err)
	}

	return SONConfigurationTransfer(append(w.Bytes(), opaque...))
}

// enbConfigTransferWire builds an ENB CONFIGURATION TRANSFER initiatingMessage
// open-type payload carrying the given SON Configuration Transfer IE, as an eNB
// would send it.
func enbConfigTransferWire(t *testing.T, son SONConfigurationTransfer) []byte {
	t.Helper()

	var w aper.Writer

	w.WriteSequencePreamble(true, false, nil)

	if err := encodeIEContainer(&w, []ieField{son.field(idSONConfigurationTransferECT)}); err != nil {
		t.Fatalf("encode IE container: %v", err)
	}

	return w.Bytes()
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

// relayedSON extracts the SON Configuration Transfer IE (id-...MCT) value from an
// MME CONFIGURATION TRANSFER body.
func relayedSON(t *testing.T, value []byte) []byte {
	t.Helper()

	r := aper.NewReader(value)

	if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
		t.Fatalf("body preamble: %v", err)
	}

	fields, err := decodeIEContainer(r)
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
	var w aper.Writer

	w.WriteSequencePreamble(true, false, nil)

	if err := encodeIEContainer(&w, nil); err != nil {
		t.Fatalf("encode empty container: %v", err)
	}

	msg, err := ParseENBConfigurationTransfer(w.Bytes())
	if err != nil {
		t.Fatalf("ParseENBConfigurationTransfer: %v", err)
	}

	if msg.SONConfigurationTransfer != nil {
		t.Fatalf("expected nil SON Configuration Transfer, got %x", msg.SONConfigurationTransfer)
	}
}
