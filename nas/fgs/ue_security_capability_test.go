// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "testing"

func TestParseUESecurityCapability(t *testing.T) {
	// 5G-EA: EA0+EA2 set (0xA0); 5G-IA: IA1+IA2 set (0x60); one E-UTRA octet.
	sc, err := ParseUESecurityCapability([]byte{0xA0, 0x60, 0xFF})
	if err != nil {
		t.Fatalf("ParseUESecurityCapability: %v", err)
	}

	if !sc.SupportsEA(0) || sc.SupportsEA(1) || !sc.SupportsEA(2) || sc.SupportsEA(3) {
		t.Errorf("EA support wrong: EA=%#x", sc.EA)
	}

	if sc.SupportsIA(0) || !sc.SupportsIA(1) || !sc.SupportsIA(2) || sc.SupportsIA(3) {
		t.Errorf("IA support wrong: IA=%#x", sc.IA)
	}

	if len(sc.Rest) != 1 || sc.Rest[0] != 0xFF {
		t.Errorf("Rest = %x", sc.Rest)
	}

	if sc.SupportsEA(8) || sc.SupportsIA(9) {
		t.Errorf("out-of-range n must be false")
	}
}

func TestParseUESecurityCapabilityTooShort(t *testing.T) {
	if _, err := ParseUESecurityCapability([]byte{0x80}); err == nil {
		t.Error("expected error for a 1-octet UE security capability")
	}
}
