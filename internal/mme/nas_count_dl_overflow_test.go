// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func sendCapturingWire(t *testing.T, ue *UeContext, plain []byte, sht eps.SecurityHeaderType) []byte {
	t.Helper()

	var wire []byte

	if err := ue.Conn().SendProtected(plain, sht, func(b []byte) error {
		wire = b

		return nil
	}); err != nil {
		t.Fatalf("SendProtected: %v", err)
	}

	return wire
}

// TS 24.301 §4.4.3.1
func TestDownlinkProtectionAdvancesCountByOne(t *testing.T) {
	ue, _ := securedUE(t, newTestMME(t))
	ue.SetDLCountForTest(10)

	sendCapturingWire(t, ue, []byte{0x07, 0x42}, eps.SHTIntegrityProtectedCiphered)

	if got := ue.DLCountForTest(); got != 11 {
		t.Fatalf("downlink NAS COUNT = %d, want 11 after one successful protect", got)
	}
}

// TS 24.301 §4.4.3.1
func TestDownlinkProtectionAppliesOverflow(t *testing.T) {
	plain := []byte{0x07, 0x42, 0x01, 0x02, 0x03}

	ue0, _ := securedUE(t, newTestMME(t))
	ue0.SetDLCountForTest(0)

	wire0 := sendCapturingWire(t, ue0, plain, eps.SHTIntegrityProtectedCiphered)

	ue256, _ := securedUE(t, newTestMME(t))
	ue256.SetDLCountForTest(256) // overflow 1, sequence number 0 — same wire SQN as COUNT 0

	wire256 := sendCapturingWire(t, ue256, plain, eps.SHTIntegrityProtectedCiphered)

	if bytes.Equal(wire0, wire256) {
		t.Fatal("downlink COUNTs 0 and 256 share sequence number 0 but must differ in the overflow counter; identical protected output means the overflow is dropped and the COUNT is reused (TS 24.301 §4.4.3.1)")
	}
}
