// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// protectedUplink builds an integrity-protected + ciphered uplink NAS message for
// the UE's security context at the given NAS COUNT. The payload is a benign EMM
// STATUS so handleNAS verifies integrity without a side-effecting procedure.
func protectedUplink(t *testing.T, ue *UeContext, count uint32) []byte {
	t.Helper()

	plain := []byte{0x07, 0x60, 0x00} // EMM PD, EMM STATUS, cause

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, count, nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

// TestNASUplinkReplayRejected checks that a protected uplink NAS message is
// accepted once and a byte-identical replay is rejected, not re-accepted
// (TS 24.301 §4.4.3, TS 33.401 §6.5).
func TestNASUplinkReplayRejected(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ulCount = 0

	msg := protectedUplink(t, ue, nascommon.NASCount(0, 0))

	m.handleNAS(context.Background(), ue, msg)

	if ue.ulCount != 1 {
		t.Fatalf("valid message not accepted: ulCount = %d, want 1", ue.ulCount)
	}

	// Replaying the identical bytes must not advance the expected count: the
	// message now estimates to a stale NAS COUNT and fails the integrity check.
	m.handleNAS(context.Background(), ue, msg)

	if ue.ulCount != 1 {
		t.Fatalf("replay accepted: ulCount advanced to %d", ue.ulCount)
	}
}

// TestNASUplinkCountWrap checks the 16-bit overflow is maintained across a
// sequence-number wrap (TS 24.301 §4.4.3.6): a message whose sequence wrapped to
// 0 verifies against NAS COUNT (overflow 1, sequence 0), not (0, 0).
func TestNASUplinkCountWrap(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ulCount = 255

	m.handleNAS(context.Background(), ue, protectedUplink(t, ue, nascommon.NASCount(0, 255)))

	if ue.ulCount != 256 {
		t.Fatalf("sequence 255 not accepted: ulCount = %d, want 256", ue.ulCount)
	}

	// The UE's sequence wraps 255->0 and its overflow becomes 1.
	m.handleNAS(context.Background(), ue, protectedUplink(t, ue, nascommon.NASCount(1, 0)))

	if ue.ulCount != 257 {
		t.Fatalf("wrapped message not accepted: ulCount = %d, want 257", ue.ulCount)
	}
}
