// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

const (
	kbps = 1
	mbps = 1_000
)

// TestEncodeAPNAMBRSpecVectors checks the encoded octets against TS 24.301
// for representative rates, including the extended-octet ranges the
// policy session-AMBR values fall in.
func TestEncodeAPNAMBRSpecVectors(t *testing.T) {
	tests := []struct {
		name           string
		dlKbps, ulKbps uint64
		wantDLBase     uint8
		wantULBase     uint8
		wantExtended   []byte // octets 5,6[,7,8]; nil = none
	}{
		// Base octet (≤8640 kbps): 576 + (v-128)*64.
		{"1 Mbps both", 1000 * kbps, 1000 * kbps, 128 + (1000-576)/64, 128 + (1000-576)/64, nil},
		// Extended octet, 17-128 Mbps range: ext = 74 + (mbps-16).
		{"30/60 Mbps", 30 * mbps, 60 * mbps, 0xFE, 0xFE, []byte{74 + (30 - 16), 74 + (60 - 16)}},
		{"100 Mbps both", 100 * mbps, 100 * mbps, 0xFE, 0xFE, []byte{74 + (100 - 16), 74 + (100 - 16)}},
		// Extended octet, 130-256 Mbps range: ext = 186 + (mbps-128)/2.
		{"200 Mbps both", 200 * mbps, 200 * mbps, 0xFE, 0xFE, []byte{186 + (200-128)/2, 186 + (200-128)/2}},
		// Mixed: DL needs extension, UL fits in the base octet (8 Mbps).
		{"50 Mbps DL / 8 Mbps UL", 50 * mbps, 8000 * kbps, 0xFE, 128 + (8000-576)/64, []byte{74 + (50 - 16), 0}},
		// Extended-2 octet (>256 Mbps). It is additive on top of octets 3 and 5:
		// 400 Mbps is one 256 Mbps step plus a 144 Mbps remainder, which octet 5
		// carries at 2 Mbps granularity. UL 100 Mbps needs no step (ext2 = 0).
		{"400 Mbps DL / 100 Mbps UL", 400 * mbps, 100 * mbps, 0xFE, 0xFE, []byte{186 + (144-128)/2, 74 + (100 - 16), 1, 0}},
		// An exact multiple of the step is one step fewer plus a full 256 Mbps
		// remainder, which is what keeps the element's maximum reachable.
		{"512 Mbps both", 512 * mbps, 512 * mbps, 0xFE, 0xFE, []byte{0xFA, 0xFA, 1, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, err := eps.APNAMBRFromKbps(tc.dlKbps, tc.ulKbps)
			if err != nil {
				t.Fatalf("APNAMBRFromKbps: %v", err)
			}

			if a.DownlinkOctet != tc.wantDLBase {
				t.Errorf("DL base octet = %#x, want %#x", a.DownlinkOctet, tc.wantDLBase)
			}

			if a.UplinkOctet != tc.wantULBase {
				t.Errorf("UL base octet = %#x, want %#x", a.UplinkOctet, tc.wantULBase)
			}

			if !bytes.Equal(a.Extended, tc.wantExtended) {
				t.Errorf("extended = % x, want % x", a.Extended, tc.wantExtended)
			}
		})
	}
}

// TestAPNAMBRRoundTrip checks encode→decode recovers the configured rate for
// values that are exactly representable at the spec granularity. These cover the
// extended-octet ranges every Ella Core policy/profile session-AMBR falls in:
// 10 Mbps (100 kbps granularity), 17-128 Mbps (1 Mbps), 130-256 Mbps (2 Mbps) —
// including the multiple_policies values (10/20/30/40/50 ↑, 50/100/150/200/250 ↓).
func TestAPNAMBRRoundTrip(t *testing.T) {
	exact := []uint64{10, 20, 30, 40, 50, 60, 100, 128, 130, 150, 200, 250, 256}

	for _, dlMbps := range exact {
		for _, ulMbps := range exact {
			dl, ul := dlMbps*mbps, ulMbps*mbps

			a, err := eps.APNAMBRFromKbps(dl, ul)
			if err != nil {
				t.Fatalf("APNAMBRFromKbps(%d, %d): %v", dl, ul, err)
			}

			gotDL, gotUL, ok := a.Kbps()
			if !ok || gotDL != dl || gotUL != ul {
				t.Errorf("round-trip %d/%d Mbps: got %d/%d kbit/s, want %d/%d", dlMbps, ulMbps, gotDL, gotUL, dl, ul)
			}
		}
	}
}

// TestAPNAMBRExtended2RoundTrip checks rates above 256 Mbps round-trip through
// the extended-2 octets. That octet adds whole 256 Mbps steps (TS 24.301
// §9.9.4.2), so a rate is exactly representable when what remains after the
// steps is itself representable by octets 3 and 5.
func TestAPNAMBRExtended2RoundTrip(t *testing.T) {
	exact := []uint64{384, 400, 456, 512, 640, 768, 1000, 1024, 2048, 5120, 65280}

	for _, dlMbps := range exact {
		for _, ulMbps := range exact {
			dl, ul := dlMbps*mbps, ulMbps*mbps

			a, err := eps.APNAMBRFromKbps(dl, ul)
			if err != nil {
				t.Fatalf("APNAMBRFromKbps(%d, %d): %v", dl, ul, err)
			}

			gotDL, gotUL, ok := a.Kbps()
			if !ok || gotDL != dl || gotUL != ul {
				t.Errorf("round-trip %d/%d Mbps: got %d/%d kbit/s, want %d/%d", dlMbps, ulMbps, gotDL, gotUL, dl, ul)
			}
		}
	}
}

// TestAPNAMBRMarshalParse round-trips the IE value bytes through MarshalBinary/Parse.
func TestAPNAMBRMarshalParse(t *testing.T) {
	orig, err := eps.APNAMBRFromKbps(60*mbps, 30*mbps)
	if err != nil {
		t.Fatalf("APNAMBRFromKbps: %v", err)
	}

	raw, err := orig.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	got, err := eps.ParseAPNAMBR(raw)
	if err != nil {
		t.Fatalf("ParseAPNAMBR: %v", err)
	}

	dl, ul, ok := got.Kbps()
	if !ok || dl != 60*mbps || ul != 30*mbps {
		t.Fatalf("after marshal/parse: dl=%d ul=%d, want 60/30 Mbps", dl, ul)
	}
}

// TestActivateDefaultBearerAPNAMBR verifies the APN-AMBR IE survives the full
// Activate Default EPS Bearer Context Request marshal/parse alongside the other
// optional IEs (ESM cause, PCO), exercising the optional-IE walker.
func TestActivateDefaultBearerAPNAMBR(t *testing.T) {
	cause := eps.ESMCause(0x32)

	ambrIE, err := eps.APNAMBRFromKbps(100*mbps, 50*mbps)
	if err != nil {
		t.Fatalf("APNAMBRFromKbps: %v", err)
	}

	pco := nas.NewProtocolConfigurationOptions([][]byte{{8, 8, 8, 8}}, 1400)

	msg := &eps.ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		PTI:                          1,
		EPSQoS:                       eps.EPSQoS{QCI: 9},
		AccessPointName:              eps.APN("internet"),
		PDNAddress:                   eps.PDNAddress{PDNType: eps.PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 1}},
		APNAMBR:                      &ambrIE,
		Cause:                        ptr(cause),
		ProtocolConfigurationOptions: &pco,
	}

	wire, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	got, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.APNAMBR == nil {
		t.Fatal("APN-AMBR IE missing after round-trip")
	}

	if dl, ul, ok := got.APNAMBR.Kbps(); !ok || dl != 100*mbps || ul != 50*mbps {
		t.Errorf("APN-AMBR = %d/%d bps, want 100/50 Mbps", dl, ul)
	}

	if got.Cause == nil || *got.Cause != cause {
		t.Errorf("ESM cause not preserved: %v", got.Cause)
	}

	if got.ProtocolConfigurationOptions == nil {
		t.Error("PCO not preserved")
	}
}
