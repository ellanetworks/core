// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"reflect"
	"testing"
)

func TestNetworkFeatureSupportRoundTrip(t *testing.T) {
	in := NetworkFeatureSupport{
		IMSVoPS3GPP: true, EMC: 2, EMF: 1, IWKN26: true, MPSI: true,
		HasOctet4: true, EMCN3: true, MCSI: true,
	}

	got, err := ParseNetworkFeatureSupport(mustBytes(in.MarshalBinary()))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round-trip = %+v, want %+v", got, in)
	}
}

// TestNetworkFeatureSupportPreservesLength confirms the element re-encodes at the
// length it arrived with, including octets this codec does not interpret.
func TestNetworkFeatureSupportPreservesLength(t *testing.T) {
	for _, raw := range [][]byte{{0x01}, {0x01, 0x03}, {0x01, 0x03, 0xAA}} {
		got, err := ParseNetworkFeatureSupport(raw)
		if err != nil {
			t.Fatalf("Parse(% x): %v", raw, err)
		}

		out, err := got.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(% x): %v", raw, err)
		}

		if !bytes.Equal(out, raw) {
			t.Errorf("round-trip % x -> % x", raw, out)
		}
	}
}

func TestNetworkFeatureSupportSingleOctet(t *testing.T) {
	// A valid 1-octet content: only IMS-VoPS-3GPP set; octet 4 fields default false.
	got, err := ParseNetworkFeatureSupport([]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}

	if !got.IMSVoPS3GPP || got.EMCN3 || got.MCSI {
		t.Fatalf("1-octet parse = %+v", got)
	}
}

func TestSelectedEPSNASSecurityAlgorithms(t *testing.T) {
	// bits 5-7 ciphering (EEA), bits 1-3 integrity (EIA): 0x21 → EEA2, EIA1.
	got, err := ParseSelectedEPSNASSecurityAlgorithms([]byte{0x21})
	if err != nil {
		t.Fatalf("ParseSelectedEPSNASSecurityAlgorithms: %v", err)
	}

	if got.Ciphering != 2 || got.Integrity != 1 {
		t.Fatalf("got %+v, want ciphering 2 / integrity 1", got)
	}

	raw, err := got.MarshalBinary()
	if err != nil || !bytes.Equal(raw, []byte{0x21}) {
		t.Fatalf("MarshalBinary = %#x err %v, want 21", raw, err)
	}
}

// TestQoSFlowOpCodes pins the operation codes to the octet positions
// TS 24.501 table 9.11.4.12.1 gives them: bits 8-6, so 0x20/0x40/0x60 on the
// wire. The constants hold the unshifted code; the codec does the shifting.
func TestQoSFlowOpCodes(t *testing.T) {
	for op, want := range map[QoSFlowOperation]byte{
		QoSFlowOpCreate: 0x20,
		QoSFlowOpDelete: 0x40,
		QoSFlowOpModify: 0x60,
	} {
		raw, err := QoSFlowDescriptions{FiveQIQoSFlow(1, 9, op)}.MarshalBinary()
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}

		// QFI octet, then the operation octet.
		if got := raw[1] & 0xE0; got != want {
			t.Errorf("%s encoded operation octet %#02x, want %#02x", op, got, want)
		}
	}
}

func TestPacketFilterComponentValueLength(t *testing.T) {
	cases := map[PacketFilterComponentType]int{0x10: 8, 0x21: 17, 0x23: 17, 0x81: 6, 0x82: 6, 0x87: 2, 0x30: 1}
	for ct, want := range cases {
		if got, ok := ct.valueLength(); !ok || got != want {
			t.Errorf("component %#x = %d,%v; want %d", ct, got, ok, want)
		}
	}
}

func TestParseGSMCapability(t *testing.T) {
	if _, err := ParseGSMCapability(nil); err == nil {
		t.Error("empty value: want error")
	}

	// RqoS (bit 1) + EPT-S1 (bit 3) + ATSSS-ST = 0b0011 (bits 4-7).
	got, err := ParseGSMCapability([]byte{0b0001_1101})
	if err != nil {
		t.Fatalf("ParseGSMCapability: %v", err)
	}

	want := GSMCapability{RqoS: true, MH6PDU: false, EPTS1: true, ATSSSST: 0b0011}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseGSMCapability = %+v; want %+v", got, want)
	}

	if _, err := ParseGSMCapability(make([]byte, maxGSMCapabilityLen+1)); err == nil {
		t.Error("over-long value: want error")
	}

	raw, err := got.MarshalBinary()
	if err != nil || !bytes.Equal(raw, []byte{0b0001_1101}) {
		t.Fatalf("MarshalBinary = %#x err %v", raw, err)
	}
}

func TestParseUPUAcknowledgement(t *testing.T) {
	mac := make([]byte, 16)
	for i := range mac {
		mac[i] = byte(i)
	}

	buf := append([]byte{UPUHeaderAck}, mac...)

	got, err := ParseUPUAcknowledgement(buf)
	if err != nil {
		t.Fatalf("ParseUPUAcknowledgement error: %v", err)
	}

	if !bytes.Equal(got.MAC[:], mac) {
		t.Errorf("MAC = % x; want % x", got.MAC, mac)
	}

	if _, err := ParseUPUAcknowledgement(buf[:16]); err == nil {
		t.Error("short buffer: want error")
	}

	bad := append([]byte{0x00}, mac...)
	if _, err := ParseUPUAcknowledgement(bad); err == nil {
		t.Error("bad UPU-Header: want error")
	}
}
