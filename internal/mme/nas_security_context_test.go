// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

// TestNextEksi verifies the eKSI cycles to a distinct value and skips 7 ("no key
// available"), so a new authentication never reuses the stored eKSI (TS 24.301
// §5.4.2.4, §9.9.3.21).
func TestNextEksi(t *testing.T) {
	for current, want := range map[uint8]uint8{0: 1, 1: 2, 5: 6, 6: 0, 7: 0} {
		if got := NextEksi(current); got != want {
			t.Errorf("NextEksi(%d) = %d, want %d", current, got, want)
		}

		if got := NextEksi(current); got == current {
			t.Errorf("NextEksi(%d) returned the same value; a new authentication must use a distinct eKSI", current)
		}
	}
}

// TestInstallNASSecurityContext_ResetsNASCounts verifies a new EPS security
// context starts both NAS COUNTs at zero, so the initial SECURITY MODE COMMAND
// rides downlink COUNT 0 (TS 24.301 §4.4.3.1).
func TestInstallNASSecurityContext_ResetsNASCounts(t *testing.T) {
	m := newTestMME(t)
	ue := m.NewUe(&captureConn{}, 7)

	ue.SetKASMEForTest(make([]byte, 32))
	ue.SetULCountForTest(5)
	ue.SetDLCountForTest(9)

	if err := ue.InstallNASSecurityContext(2, 2, MintAuthProofForSecurityMode()); err != nil {
		t.Fatalf("InstallNASSecurityContext: %v", err)
	}

	if got := ue.ULCount(); got != 0 {
		t.Errorf("uplink NAS COUNT = %d, want 0 after installing a new security context", got)
	}

	if got := ue.DLCountForTest(); got != 0 {
		t.Errorf("downlink NAS COUNT = %d, want 0 after installing a new security context", got)
	}
}
