// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "testing"

// TS 24.501 §4.4.3.2
func TestNASUplinkReplayRejected(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	msg := wrapProtected(t, ue, encodePlainULNasTransport(t), 0)

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

	if _, err := DecodeNASMessage(ue, msg); err == nil {
		t.Fatal("replay accepted: a NAS COUNT must be accepted at most one time")
	}

	if ue.ULCount() != 1 {
		t.Fatalf("replay advanced ulCount to %d, want 1", ue.ULCount())
	}
}

// TS 24.501 §4.4.3.1
func TestNASUplinkCountWrap(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(255)

	if _, err := DecodeNASMessage(ue, wrapProtected(t, ue, encodePlainULNasTransport(t), 255)); err != nil {
		t.Fatalf("sequence 255 not accepted: %v", err)
	}

	if ue.ULCount() != 256 {
		t.Fatalf("after sequence 255 ulCount = %d, want 256", ue.ULCount())
	}

	if _, err := DecodeNASMessage(ue, wrapProtected(t, ue, encodePlainULNasTransport(t), 0)); err != nil {
		t.Fatalf("wrapped message not accepted: %v", err)
	}

	if ue.ULCount() != 257 {
		t.Fatalf("after the wrapped message ulCount = %d, want 257", ue.ULCount())
	}
}
