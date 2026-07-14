// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import "testing"

// TestNASUplinkReplayRejected checks that a protected uplink NAS message is
// accepted once and a byte-identical replay is rejected, not re-accepted
// (TS 24.501 §4.4.3.2, TS 33.501 §6.4.3).
func TestNASUplinkReplayRejected(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	msg := wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 0)

	res, err := DecodeNASMessage(ue, msg)
	if err != nil {
		t.Fatalf("valid message not accepted: %v", err)
	}

	if !res.IntegrityVerified {
		t.Fatal("valid message not integrity-verified")
	}

	if ue.ULCount() != 1 {
		t.Fatalf("after the accepted message ulCount = %d, want 1", ue.ULCount())
	}

	// The replay estimates to a NAS COUNT past the accepted one, so its MAC does
	// not verify and the message is discarded.
	if _, err := DecodeNASMessage(ue, msg); err == nil {
		t.Fatal("replay accepted: a NAS COUNT must be accepted at most one time")
	}

	if ue.ULCount() != 1 {
		t.Fatalf("replay advanced ulCount to %d, want 1", ue.ULCount())
	}
}

// TestNASUplinkCountWrap checks the 16-bit overflow is maintained across a
// sequence-number wrap (TS 24.501 §4.4.3.1): a message whose sequence wrapped to
// 0 verifies against NAS COUNT (overflow 1, sequence 0), not (0, 0).
func TestNASUplinkCountWrap(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(255)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 255)); err != nil {
		t.Fatalf("sequence 255 not accepted: %v", err)
	}

	if ue.ULCount() != 256 {
		t.Fatalf("after sequence 255 ulCount = %d, want 256", ue.ULCount())
	}

	// The UE's sequence wraps 255->0 and its overflow becomes 1.
	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 0)); err != nil {
		t.Fatalf("wrapped message not accepted: %v", err)
	}

	if ue.ULCount() != 257 {
		t.Fatalf("after the wrapped message ulCount = %d, want 257", ue.ULCount())
	}
}
