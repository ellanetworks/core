// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"net/netip"
	"testing"
)

func TestIPToIn6Addr_IPv4(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.1")
	got := IPToIn6Addr(addr)

	// IPv4-mapped IPv6: bytes 0-9 = 0x00, bytes 10-11 = 0xff, bytes 12-15 = IPv4
	want := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 168, 1, 1}
	if got != want {
		t.Errorf("IPToIn6Addr(IPv4): got %v, want %v", got, want)
	}
}

func TestIPToIn6Addr_IPv6(t *testing.T) {
	addr := netip.MustParseAddr("2001:db8::1")
	got := IPToIn6Addr(addr)
	want := addr.As16()

	if got != want {
		t.Errorf("IPToIn6Addr(IPv6): got %v, want %v", got, want)
	}
}

func TestIn6AddrToIP_IPv4Mapped(t *testing.T) {
	in6 := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 10, 0, 0, 1}
	got := In6AddrToIP(in6)
	want := netip.MustParseAddr("10.0.0.1")

	if got != want {
		t.Errorf("In6AddrToIP(IPv4-mapped): got %v, want %v", got, want)
	}
}

func TestIn6AddrToIP_IPv6Native(t *testing.T) {
	want := netip.MustParseAddr("2001:db8::cafe")
	in6 := IPToIn6Addr(want)
	got := In6AddrToIP(in6)

	if got != want {
		t.Errorf("In6AddrToIP(IPv6): got %v, want %v", got, want)
	}
}

func TestIPToIn6Addr_RoundTrip_IPv4(t *testing.T) {
	original := netip.MustParseAddr("172.16.0.42")
	got := In6AddrToIP(IPToIn6Addr(original))

	if got != original {
		t.Errorf("round-trip IPv4: got %v, want %v", got, original)
	}
}

func TestIPToIn6Addr_RoundTrip_IPv6(t *testing.T) {
	original := netip.MustParseAddr("fd00::1")
	got := In6AddrToIP(IPToIn6Addr(original))

	if got != original {
		t.Errorf("round-trip IPv6: got %v, want %v", got, original)
	}
}

// The conversion fills the identifiers first and the FAR and QER after, so
// returning early on the IMSI leaves a rule with FAR action 0 — one that forwards
// nowhere, on a session that looks established.
func TestToN3N6EntrypointPdrInfoRejectsBadIMSI(t *testing.T) {
	for _, imsi := range []string{"", "not-a-number", "00110070001234x"} {
		if _, err := ToN3N6EntrypointPdrInfo(PdrInfo{
			SEID:  1,
			PdrID: 2,
			IMSI:  imsi,
			Far:   FarInfo{Action: 2},
			Qer:   QerInfo{Qfi: 9},
		}); err == nil {
			t.Errorf("IMSI %q: converted without error, want a rejection", imsi)
		}
	}
}

func TestToN3N6EntrypointPdrInfoCarriesFARAndQER(t *testing.T) {
	got, err := ToN3N6EntrypointPdrInfo(PdrInfo{
		SEID:  1,
		PdrID: 2,
		IMSI:  "001010000000001",
		Far:   FarInfo{Action: 2, TeID: 0x1234},
		Qer:   QerInfo{Qfi: 9, MaxBitrateUL: 1000},
	})
	if err != nil {
		t.Fatalf("ToN3N6EntrypointPdrInfo: %v", err)
	}

	if got.Imsi != 1010000000001 {
		t.Errorf("Imsi = %d, want 1010000000001", got.Imsi)
	}

	if got.Far.Action != 2 || got.Far.Teid != 0x1234 {
		t.Errorf("FAR = %+v, want action 2 teid 0x1234", got.Far)
	}

	if got.Qer.Qfi != 9 || got.Qer.UlMaximumBitrate != 1000 {
		t.Errorf("QER = %+v, want qfi 9 ul 1000", got.Qer)
	}
}
