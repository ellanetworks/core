// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

const (
	kbps = 1_000
	mbps = 1_000_000
)

// TestEncodeAPNAMBRSpecVectors checks the encoded octets against TS 24.301
// §9.9.4.2 for representative rates, including the extended-octet ranges the
// policy session-AMBR values fall in.
func TestEncodeAPNAMBRSpecVectors(t *testing.T) {
	tests := []struct {
		name           string
		dlBps, ulBps   uint64
		wantDLBase     uint8
		wantULBase     uint8
		wantExtPresent bool
		wantDLExt      uint8
		wantULExt      uint8
	}{
		// Base octet (≤8640 kbps): 576 + (v-128)*64.
		{"1 Mbps both", 1000 * kbps, 1000 * kbps, 128 + (1000-576)/64, 128 + (1000-576)/64, false, 0, 0},
		// Extended octet, 17-128 Mbps range: ext = 74 + (mbps-16).
		{"30/60 Mbps", 30 * mbps, 60 * mbps, 0xFE, 0xFE, true, 74 + (30 - 16), 74 + (60 - 16)},
		{"100 Mbps both", 100 * mbps, 100 * mbps, 0xFE, 0xFE, true, 74 + (100 - 16), 74 + (100 - 16)},
		// Extended octet, 130-256 Mbps range: ext = 186 + (mbps-128)/2.
		{"200 Mbps both", 200 * mbps, 200 * mbps, 0xFE, 0xFE, true, 186 + (200-128)/2, 186 + (200-128)/2},
		// Mixed: DL needs extension, UL fits in the base octet (8 Mbps).
		{"50 Mbps DL / 8 Mbps UL", 50 * mbps, 8000 * kbps, 0xFE, 128 + (8000-576)/64, true, 74 + (50 - 16), 0},
		// Clamp above 256 Mbps.
		{"1 Gbps both clamps to 256", 1000 * mbps, 1000 * mbps, 0xFE, 0xFE, true, 0xFA, 0xFA},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := eps.EncodeAPNAMBR(tc.dlBps, tc.ulBps)

			if a.DownlinkOctet != tc.wantDLBase {
				t.Errorf("DL base octet = %#x, want %#x", a.DownlinkOctet, tc.wantDLBase)
			}

			if a.UplinkOctet != tc.wantULBase {
				t.Errorf("UL base octet = %#x, want %#x", a.UplinkOctet, tc.wantULBase)
			}

			if tc.wantExtPresent {
				if len(a.Extended) != 2 {
					t.Fatalf("expected 2 extended octets, got %d", len(a.Extended))
				}

				if a.Extended[0] != tc.wantDLExt || a.Extended[1] != tc.wantULExt {
					t.Errorf("extended = [%#x %#x], want [%#x %#x]", a.Extended[0], a.Extended[1], tc.wantDLExt, tc.wantULExt)
				}
			} else if len(a.Extended) != 0 {
				t.Errorf("expected no extended octets, got %d", len(a.Extended))
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

			gotDL, gotUL := eps.EncodeAPNAMBR(dl, ul).BitsPerSecond()

			if gotDL != dl || gotUL != ul {
				t.Errorf("round-trip %d/%d Mbps: got %d/%d bps, want %d/%d", dlMbps, ulMbps, gotDL, gotUL, dl, ul)
			}
		}
	}
}

// TestAPNAMBRClampDecodes verifies a >256 Mbps rate decodes to the clamp value.
func TestAPNAMBRClampDecodes(t *testing.T) {
	dl, _ := eps.EncodeAPNAMBR(1000*mbps, 1000*mbps).BitsPerSecond()
	if dl != 256*mbps {
		t.Fatalf("clamped DL = %d bps, want %d (256 Mbps)", dl, 256*mbps)
	}
}

// TestAPNAMBRMarshalParse round-trips the IE value bytes through Marshal/Parse.
func TestAPNAMBRMarshalParse(t *testing.T) {
	orig := eps.EncodeAPNAMBR(60*mbps, 30*mbps)

	got, err := eps.ParseAPNAMBR(orig.Marshal())
	if err != nil {
		t.Fatalf("ParseAPNAMBR: %v", err)
	}

	dl, ul := got.BitsPerSecond()
	if dl != 60*mbps || ul != 30*mbps {
		t.Fatalf("after marshal/parse: dl=%d ul=%d, want 60/30 Mbps", dl, ul)
	}
}

// TestActivateDefaultBearerAPNAMBR verifies the APN-AMBR IE survives the full
// Activate Default EPS Bearer Context Request marshal/parse alongside the other
// optional IEs (ESM cause, PCO), exercising the optional-IE walker.
func TestActivateDefaultBearerAPNAMBR(t *testing.T) {
	cause := uint8(0x32)
	msg := &eps.ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 1,
		EPSQoS:                       []byte{9},
		AccessPointName:              []byte{0x08, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'},
		PDNAddress:                   []byte{0x01, 10, 45, 0, 1},
		APNAMBR:                      eps.EncodeAPNAMBR(100*mbps, 50*mbps).Marshal(),
		ESMCause:                     &cause,
		ProtocolConfigurationOptions: []byte{0x80, 0x80, 0x21, 0x02},
	}

	wire, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(got.APNAMBR) == 0 {
		t.Fatal("APN-AMBR IE missing after round-trip")
	}

	ambr, err := eps.ParseAPNAMBR(got.APNAMBR)
	if err != nil {
		t.Fatalf("ParseAPNAMBR: %v", err)
	}

	if dl, ul := ambr.BitsPerSecond(); dl != 100*mbps || ul != 50*mbps {
		t.Errorf("APN-AMBR = %d/%d bps, want 100/50 Mbps", dl, ul)
	}

	if got.ESMCause == nil || *got.ESMCause != cause {
		t.Errorf("ESM cause not preserved: %v", got.ESMCause)
	}

	if len(got.ProtocolConfigurationOptions) == 0 {
		t.Error("PCO not preserved")
	}
}
