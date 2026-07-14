// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import "testing"

func TestUplinkCounterFirstMessageIsZero(t *testing.T) {
	var u UplinkCounter

	if got := u.NextExpected(); got != 0 {
		t.Fatalf("NextExpected() = %d, want 0", got)
	}

	if got := u.Estimate(0); got != 0 {
		t.Fatalf("Estimate(0) = %d, want 0", got)
	}
}

func TestUplinkCounterEstimateAdvances(t *testing.T) {
	var u UplinkCounter

	u.Commit(u.Estimate(0))

	if got := u.Estimate(1); got != MakeCount(0, 1) {
		t.Fatalf("Estimate(1) = (%d,%d), want (0,1)", got.Overflow(), got.SQN())
	}
}

// A replayed message carries the sequence number of an already accepted one. Its
// estimate must not be the count it was accepted at, or its MAC would verify a
// second time (TS 24.301 §4.4.3.2, TS 24.501 §4.4.3.2).
func TestUplinkCounterEstimateRejectsReplay(t *testing.T) {
	var u UplinkCounter

	u.Commit(MakeCount(7, 10))

	if got := u.Estimate(10); got == MakeCount(7, 10) {
		t.Fatalf("Estimate(10) = (%d,%d), want any count other than the accepted (7,10)", got.Overflow(), got.SQN())
	}
}

func TestUplinkCounterEstimateWraps(t *testing.T) {
	var u UplinkCounter

	u.Commit(MakeCount(7, 255))

	if got := u.Estimate(0); got != MakeCount(8, 0) {
		t.Fatalf("Estimate(0) = (%d,%d), want (8,0)", got.Overflow(), got.SQN())
	}
}

func TestUplinkCounterLastAccepted(t *testing.T) {
	var u UplinkCounter

	if got := u.LastAccepted(); got != 0 {
		t.Fatalf("LastAccepted() = %d, want 0 before any message", got)
	}

	u.Commit(MakeCount(3, 9))

	if got := u.LastAccepted(); got != MakeCount(3, 9) {
		t.Fatalf("LastAccepted() = (%d,%d), want (3,9)", got.Overflow(), got.SQN())
	}
}

func TestUplinkCounterResetExpectsZero(t *testing.T) {
	var u UplinkCounter

	u.Commit(MakeCount(3, 9))
	u.Reset()

	if got := u.NextExpected(); got != 0 {
		t.Fatalf("NextExpected() after Reset() = %d, want 0", got)
	}

	if got := u.Estimate(0); got != 0 {
		t.Fatalf("Estimate(0) after Reset() = %d, want 0", got)
	}
}
