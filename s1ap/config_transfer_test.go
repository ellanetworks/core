// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

func sonTransfer(t *testing.T, target TargeteNBID) SONConfigurationTransfer {
	t.Helper()

	tla := tlaOf(192, 0, 2, 7)

	son := SONConfigurationTransfer{
		TargeteNBID: target,
		SourceeNBID: SourceeNBID{
			GlobalENBID: GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{MacroENBID: Ptr(MacroENBID(0x00abd))}},
			SelectedTAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 8},
		},
		SONInformation:         SONInformation{SONInformationRequest: Ptr(SONInformationRequestX2TNLConfigurationInfo)},
		X2TNLConfigurationInfo: &X2TNLConfigurationInfo{ENBX2TransportLayerAddresses: ENBX2TLAs{tla}},
		RequestedTNLInfo:       &RequestedTNLInfo{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}},
	}
	son.iEExtensions.Append(rawIE{ID: 40123, Criticality: CriticalityIgnore, Value: []byte{0xde, 0xad}})

	return son
}

func enbConfigTransferWire(t *testing.T, son *SONConfigurationTransfer) []byte {
	t.Helper()

	w := per.NewWriter()

	w.WriteBit(false)

	if err := encodeIEContainer(w, per.Aligned, []ieField{{ID: IDSONConfigurationTransferECT, Criticality: CriticalityIgnore, Value: son}}); err != nil {
		t.Fatalf("encode IE container: %v", err)
	}

	return perBytes(w)
}

func TestENBConfigurationTransfer_RelayRoundTrip(t *testing.T) {
	target := TargeteNBID{
		GlobalENBID: GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{MacroENBID: Ptr(MacroENBID(0x00abc))}},
		SelectedTAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
	}
	son := sonTransfer(t, target)
	sonWire := ieRaw(t, &son)

	// Decode the ENB CONFIGURATION TRANSFER as the MME receives it.
	msg, err := ParseENBConfigurationTransfer(enbConfigTransferWire(t, &son))
	if err != nil {
		t.Fatalf("ParseENBConfigurationTransfer: %v", err)
	}

	if msg.SONConfigurationTransfer == nil {
		t.Fatal("SON Configuration Transfer IE missing")
	}

	if got := ieRaw(t, msg.SONConfigurationTransfer); !bytes.Equal(got, sonWire) {
		t.Fatalf("SON value not preserved: got %x want %x", got, sonWire)
	}

	if msg.SONConfigurationTransfer.X2TNLConfigurationInfo == nil || msg.SONConfigurationTransfer.RequestedTNLInfo == nil {
		t.Fatalf("typed extensions lost: %+v", msg.SONConfigurationTransfer)
	}

	// Routing: the leading Target eNB-ID must decode to the destination eNB.
	got := msg.SONConfigurationTransfer.TargeteNBID

	if !got.GlobalENBID.Equal(target.GlobalENBID) || got.SelectedTAI != target.SelectedTAI {
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
	if !bytes.Equal(relayed, sonWire) {
		t.Fatalf("relayed SON not verbatim: got %x want %x", relayed, sonWire)
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
		if f.ID == IDSONConfigurationTransferMCT {
			return f.Value
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
		t.Fatalf("expected nil SON Configuration Transfer, got %+v", msg.SONConfigurationTransfer)
	}
}

// §8.15.2/§8.16.2 relay the transfer verbatim, so the payload must survive
// untouched in both directions.
func TestConfigurationTransferRelaysVerbatimBothDirections(t *testing.T) {
	target := TargeteNBID{
		GlobalENBID: GlobalENBID{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: ENBID{HomeENBID: Ptr(HomeENBID(0x1234567))}},
		SelectedTAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 9},
	}
	son := sonTransfer(t, target)
	transfer := &son

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

		if got, want := ieRaw(t, out.SONConfigurationTransfer), ieRaw(t, transfer); !bytes.Equal(got, want) {
			t.Errorf("transfer = %x, want %x", got, want)
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

		if got, want := ieRaw(t, out.SONConfigurationTransfer), ieRaw(t, transfer); !bytes.Equal(got, want) {
			t.Errorf("transfer = %x, want %x", got, want)
		}
	})
}
