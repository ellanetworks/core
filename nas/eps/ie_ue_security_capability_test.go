// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// replayed encodes the capability the MME would echo from the wire form of the
// two capabilities the UE sent, failing the test if any step errors.
func replayed(t *testing.T, ueNetCap, msNetCap []byte) []byte {
	t.Helper()

	netCap, err := eps.ParseUENetworkCapability(ueNetCap)
	if err != nil {
		t.Fatalf("ParseUENetworkCapability(% x): %v", ueNetCap, err)
	}

	var ms *eps.MSNetworkCapability

	if msNetCap != nil {
		parsed, err := eps.ParseMSNetworkCapability(msNetCap)
		if err != nil {
			t.Fatalf("ParseMSNetworkCapability(% x): %v", msNetCap, err)
		}

		ms = &parsed
	}

	raw, err := eps.ReplayedUESecurityCapability(netCap, ms).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	return raw
}

// TestReplayedUESecurityCapabilityClearsUCS2 reproduces the iPhone Security Mode
// Reject (cause #23): the UE network capability octet 6 bit 8 is UCS2 support,
// but the same bit is spare in the UE security capability the SMC replays
// (TS 24.301 §9.9.3.34 vs §9.9.3.36). A UE that set UCS2 rejects the SMC unless
// the replay clears it.
func TestReplayedUESecurityCapabilityClearsUCS2(t *testing.T) {
	// EEA=0x80, EIA=0x80, octet5 UEA=0xc0, octet6 = UCS2(bit8) | UIA1(bit7) = 0xc0.
	ueNetCap := []byte{0x80, 0x80, 0xc0, 0xc0}

	got := replayed(t, ueNetCap, nil)
	want := []byte{0x80, 0x80, 0xc0, 0x40} // octet 6 UCS2 cleared, UIA1 retained

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecurityCapability(% x) = % x, want % x (octet 6 bit 8 must be cleared)", ueNetCap, got, want)
	}
}

// TestReplayedUESecurityCapabilityMinimal covers a UE that sends only the EEA/EIA
// octets (e.g. the srsUE test SIM): the replay is the two algorithm octets, no
// UMTS octets appended.
func TestReplayedUESecurityCapabilityMinimal(t *testing.T) {
	got := replayed(t, []byte{0xe0, 0xe0}, nil)
	want := []byte{0xe0, 0xe0}

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecurityCapability = % x, want % x", got, want)
	}
}

// TestReplayedUESecurityCapabilityGERAN covers a UE that advertised GERAN
// ciphering in its MS network capability: octet 7 (GEA) is appended per
// TS 24.301 §9.9.3.36 / §5.4.3.2, with GEA1 at bit 7 and GEA2 at bit 6.
func TestReplayedUESecurityCapabilityGERAN(t *testing.T) {
	ueNetCap := []byte{0x80, 0x80, 0xc0, 0x40}   // EEA, EIA, UEA, UIA (no UCS2)
	msNetCap := []byte{0x80, 0x40}               // GEA1 (octet1 bit8), GEA2 (octet2 bit7)
	want := []byte{0x80, 0x80, 0xc0, 0x40, 0x60} // octet 7 = GEA1(bit7) | GEA2(bit6)

	got := replayed(t, ueNetCap, msNetCap)

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecurityCapability = % x, want % x", got, want)
	}
}

// TestReplayedUESecurityCapabilityGERANNoUMTS covers a UE that advertised GERAN
// but no UMTS algorithms: octets 5-6 are present and zero-filled ahead of octet 7
// (TS 24.301 §9.9.3.36).
func TestReplayedUESecurityCapabilityGERANNoUMTS(t *testing.T) {
	got := replayed(t, []byte{0xe0, 0xe0}, []byte{0x80, 0x00})
	want := []byte{0xe0, 0xe0, 0x00, 0x00, 0x40} // GEA1 only

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecurityCapability = % x, want % x", got, want)
	}
}

// TestReplayedUESecurityCapabilityNoGERAN confirms an all-zero GEA bitmap (UE
// supports no Gb-mode algorithm) omits octet 7 (TS 24.301 §9.9.3.36).
func TestReplayedUESecurityCapabilityNoGERAN(t *testing.T) {
	got := replayed(t, []byte{0x80, 0x80, 0xc0, 0x40}, []byte{0x00, 0x00})
	want := []byte{0x80, 0x80, 0xc0, 0x40}

	if !bytes.Equal(got, want) {
		t.Fatalf("ReplayedUESecurityCapability = % x, want % x", got, want)
	}
}

// TestUESecurityCapabilityRoundTrip confirms each spec-permitted length decodes
// and re-encodes byte-for-byte (TS 24.301 §9.9.3.36).
func TestUESecurityCapabilityRoundTrip(t *testing.T) {
	for _, raw := range [][]byte{
		{0xe0, 0xe0},
		{0x80, 0x80, 0xc0, 0x40},
		{0x80, 0x80, 0xc0, 0x40, 0x60},
	} {
		c, err := eps.ParseUESecurityCapability(raw)
		if err != nil {
			t.Fatalf("Parse(% x): %v", raw, err)
		}

		got, err := c.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(% x): %v", raw, err)
		}

		if !bytes.Equal(got, raw) {
			t.Errorf("round-trip % x -> % x", raw, got)
		}
	}

	for _, raw := range [][]byte{{}, {0x01}, {0x01, 0x02, 0x03}, {0x01, 0x02, 0x03, 0x04, 0x05, 0x06}} {
		if _, err := eps.ParseUESecurityCapability(raw); err == nil {
			t.Errorf("Parse(% x): want error for a length the spec does not define", raw)
		}
	}
}
