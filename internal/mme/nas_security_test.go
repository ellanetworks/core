// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
)

// TestReplayedUESecCapClearsUCS2 reproduces the iPhone Security Mode Reject
// (cause #23): the UE network capability octet 6 bit 8 is UCS2 support, but the
// same bit is spare in the UE security capability the SMC replays (TS 24.301
// §9.9.3.34 vs §9.9.3.36). A UE that set UCS2 rejects the SMC unless the replay
// clears it.
func TestReplayedUESecCapClearsUCS2(t *testing.T) {
	// EEA=0x80, EIA=0x80, octet5 UEA=0xc0, octet6 = UCS2(bit8) | UIA1(bit7) = 0xc0.
	ueNetCap := []byte{0x80, 0x80, 0xc0, 0xc0}

	got := ReplayedUESecCap(ueNetCap, nil)
	want := []byte{0x80, 0x80, 0xc0, 0x40} // octet 6 UCS2 cleared, UIA1 retained

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecCap(% x) = % x, want % x (octet 6 bit 8 must be cleared)", ueNetCap, got, want)
	}
}

// TestReplayedUESecCapMinimal covers a UE that sends only the EEA/EIA octets
// (e.g. the srsUE test SIM): the replay is the two algorithm octets, no UMTS
// octets appended.
func TestReplayedUESecCapMinimal(t *testing.T) {
	got := ReplayedUESecCap([]byte{0xe0, 0xe0}, nil)
	want := []byte{0xe0, 0xe0}

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecCap = % x, want % x", got, want)
	}
}

// TestReplayedUESecCapGERAN covers a UE that advertised GERAN ciphering in its
// MS network capability: octet 7 (GEA) is appended per TS 24.301 §9.9.3.36 /
// §5.4.3.2, with GEA1 at bit 7 and GEA2 at bit 6.
func TestReplayedUESecCapGERAN(t *testing.T) {
	ueNetCap := []byte{0x80, 0x80, 0xc0, 0x40}   // EEA, EIA, UEA, UIA (no UCS2)
	msNetCap := []byte{0x80, 0x40}               // GEA1 (octet1 bit8), GEA2 (octet2 bit7)
	want := []byte{0x80, 0x80, 0xc0, 0x40, 0x60} // octet 7 = GEA1(bit7) | GEA2(bit6)

	got := ReplayedUESecCap(ueNetCap, msNetCap)

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecCap = % x, want % x", got, want)
	}
}

// TestReplayedUESecCapGERANNoUMTS covers a UE that advertised GERAN but no UMTS
// algorithms: octets 5-6 are present and zero-filled ahead of octet 7
// (TS 24.301 §9.9.3.36).
func TestReplayedUESecCapGERANNoUMTS(t *testing.T) {
	got := ReplayedUESecCap([]byte{0xe0, 0xe0}, []byte{0x80, 0x00})
	want := []byte{0xe0, 0xe0, 0x00, 0x00, 0x40} // GEA1 only

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecCap = % x, want % x", got, want)
	}
}

// TestReplayedUESecCapNoGERAN confirms an all-zero GEA bitmap (UE supports no
// Gb-mode algorithm) omits octet 7 (TS 24.301 §9.9.3.36).
func TestReplayedUESecCapNoGERAN(t *testing.T) {
	got := ReplayedUESecCap([]byte{0x80, 0x80, 0xc0, 0x40}, []byte{0x00, 0x00})
	want := []byte{0x80, 0x80, 0xc0, 0x40}

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecCap = % x, want % x", got, want)
	}
}

func TestSelectEPSAlgorithm(t *testing.T) {
	supportsAll := func(uint8) bool { return true }
	supportsAESOnly := func(n uint8) bool { return n == 2 }
	supportsNullOnly := func(n uint8) bool { return n == 0 }

	tests := []struct {
		name       string
		preference []string
		supported  func(uint8) bool
		want       byte
		wantOK     bool
	}{
		{"AES preferred", []string{"AES", "SNOW3G"}, supportsAll, 2, true},
		{"SNOW3G preferred", []string{"SNOW3G", "AES"}, supportsAll, 1, true},
		{"SNOW3G preferred but UE lacks it", []string{"SNOW3G", "AES"}, supportsAESOnly, 2, true},
		{"no common algorithm", []string{"SNOW3G"}, supportsAESOnly, 0, false},
		{"NULL configured and UE advertises it", []string{"AES", "NULL"}, supportsNullOnly, 0, true},
		{"NULL configured but UE does not advertise it", []string{"AES", "NULL"}, supportsAESOnly, 2, true},
		{"NULL configured, UE supports nothing", []string{"NULL"}, supportsAESOnly, 0, false},
		{"empty preference", nil, supportsAll, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := selectEPSAlgorithm(tt.preference, tt.supported)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("selectEPSAlgorithm = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// A UE that advertises no integrity algorithm common with the operator policy is
// rejected (EMM cause #23), not silently downgraded to the null algorithm
// (TS 33.401 §5).
func TestCipherIntegrityAlgMapping(t *testing.T) {
	if _, ok := CipherAlg(1).(nascommon.SNOW3GCipher); !ok {
		t.Errorf("CipherAlg(1) = %T, want SNOW3GCipher", CipherAlg(1))
	}

	if _, ok := CipherAlg(2).(nascommon.AESCTRCipher); !ok {
		t.Errorf("CipherAlg(2) = %T, want AESCTRCipher", CipherAlg(2))
	}

	if _, ok := CipherAlg(0).(nascommon.NullCipher); !ok {
		t.Errorf("CipherAlg(0) = %T, want NullCipher", CipherAlg(0))
	}

	if _, ok := IntegrityAlg(1).(nascommon.SNOW3GIntegrity); !ok {
		t.Errorf("IntegrityAlg(1) = %T, want SNOW3GIntegrity", IntegrityAlg(1))
	}

	if _, ok := IntegrityAlg(2).(nascommon.AESCMACIntegrity); !ok {
		t.Errorf("IntegrityAlg(2) = %T, want AESCMACIntegrity", IntegrityAlg(2))
	}

	if _, ok := IntegrityAlg(0).(nascommon.NullIntegrity); !ok {
		t.Errorf("IntegrityAlg(0) = %T, want NullIntegrity", IntegrityAlg(0))
	}
}
