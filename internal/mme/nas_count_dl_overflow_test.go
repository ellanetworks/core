// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TestProtectDownlink_AdvancesCountByOne verifies a successful protect advances
// the downlink NAS COUNT by exactly one, using the pre-increment value for the
// message (TS 24.301 §4.4.3.1). The protection-failure path (COUNT not consumed)
// is unreachable here — eps.Protect does not fail for a fixed-size key and a
// valid algorithm — so only the success contract is asserted.
func TestProtectDownlink_AdvancesCountByOne(t *testing.T) {
	ue, _ := securedUE(t, newTestMME(t))
	ue.SetDLCountForTest(10)

	if _, err := ue.ProtectDownlink([]byte{0x07, 0x42}, eps.SHTIntegrityProtectedCiphered); err != nil {
		t.Fatalf("ProtectDownlink: %v", err)
	}

	if got := ue.DLCountForTest(); got != 11 {
		t.Fatalf("downlink NAS COUNT = %d, want 11 after one successful protect", got)
	}
}

// TestProtectDownlinkAppliesOverflow is the regression for the downlink NAS
// COUNT overflow: two downlink messages 256 apart carry the same 8-bit sequence
// number but differ in the 16-bit overflow counter, so the NAS COUNT input to
// integrity and ciphering differs and the protected output must differ
// (TS 24.301 §4.4.3.1). Dropping the overflow reuses the COUNT once the sequence
// number wraps, which a spec-compliant UE rejects.
func TestProtectDownlinkAppliesOverflow(t *testing.T) {
	plain := []byte{0x07, 0x42, 0x01, 0x02, 0x03}

	ue0, _ := securedUE(t, newTestMME(t))
	ue0.SetDLCountForTest(0)

	wire0, err := ue0.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		t.Fatalf("ProtectDownlink at COUNT 0: %v", err)
	}

	ue256, _ := securedUE(t, newTestMME(t))
	ue256.SetDLCountForTest(256) // overflow 1, sequence number 0 — same wire SQN as COUNT 0

	wire256, err := ue256.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		t.Fatalf("ProtectDownlink at COUNT 256: %v", err)
	}

	if bytes.Equal(wire0, wire256) {
		t.Fatal("downlink COUNTs 0 and 256 share sequence number 0 but must differ in the overflow counter; identical protected output means the overflow is dropped and the COUNT is reused (TS 24.301 §4.4.3.1)")
	}
}
