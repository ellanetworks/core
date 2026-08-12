// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"
	"testing"
)

func TestUplinkCounterFirstMessageIsZero(t *testing.T) {
	var u UplinkCounter

	if got := u.NextExpected(); got != 0 {
		t.Fatalf("NextExpected() = %d, want 0", got)
	}

	if got := mustEstimate(t, u, 0); got != 0 {
		t.Fatalf("Estimate(0) = %d, want 0", got)
	}
}

func TestUplinkCounterEstimateAdvances(t *testing.T) {
	var u UplinkCounter

	mustCommit(t, &u, mustEstimate(t, u, 0))

	if got := mustEstimate(t, u, 1); got != MakeCount(0, 1) {
		t.Fatalf("Estimate(1) = (%d,%d), want (0,1)", got.Overflow(), got.SQN())
	}
}

// A replayed message carries the sequence number of an already accepted one. Its
// estimate must not be the count it was accepted at, or its MAC would verify a
// second time (TS 24.301 §4.4.3.2, TS 24.501 §4.4.3.2).
func TestUplinkCounterEstimateRejectsReplay(t *testing.T) {
	u := NewUplinkCounter(MakeCount(7, 10))

	if got := mustEstimate(t, u, 10); got == MakeCount(7, 10) {
		t.Fatalf("Estimate(10) = (%d,%d), want any count other than the accepted (7,10)", got.Overflow(), got.SQN())
	}
}

func TestUplinkCounterEstimateWraps(t *testing.T) {
	u := NewUplinkCounter(MakeCount(7, 255))

	if got := mustEstimate(t, u, 0); got != MakeCount(8, 0) {
		t.Fatalf("Estimate(0) = (%d,%d), want (8,0)", got.Overflow(), got.SQN())
	}
}

func TestUplinkCounterLastAccepted(t *testing.T) {
	var u UplinkCounter

	if got := u.LastAccepted(); got != 0 {
		t.Fatalf("LastAccepted() = %d, want 0 before any message", got)
	}

	if u.Accepted() {
		t.Fatal("Accepted() is true before any message")
	}

	mustCommit(t, &u, MakeCount(0, 9))

	if got := u.LastAccepted(); got != MakeCount(0, 9) {
		t.Fatalf("LastAccepted() = (%d,%d), want (0,9)", got.Overflow(), got.SQN())
	}

	if !u.Accepted() {
		t.Fatal("Accepted() is false after a commit")
	}
}

func TestUplinkCounterResetExpectsZero(t *testing.T) {
	u := NewUplinkCounter(MakeCount(3, 9))
	u.Reset()

	if got := u.NextExpected(); got != 0 {
		t.Fatalf("NextExpected() after Reset() = %d, want 0", got)
	}

	if got := mustEstimate(t, u, 0); got != 0 {
		t.Fatalf("Estimate(0) after Reset() = %d, want 0", got)
	}
}

// mustCommit records a count the counter must accept.
func mustCommit(t *testing.T, u *UplinkCounter, count Count) {
	t.Helper()

	if err := u.Commit(count); err != nil {
		t.Fatalf("Commit(%#06x): %v", uint32(count), err)
	}
}

// TestUplinkCounterCommitRejectsUnexpected checks that a count Estimate could
// not have produced is refused: committing one would move the replay window
// somewhere the check never sanctioned.
func TestUplinkCounterCommitRejectsUnexpected(t *testing.T) {
	u := NewUplinkCounter(MakeCount(4, 9))

	if err := u.Commit(MakeCount(4, 10)); err != nil {
		t.Fatalf("the next count in sequence was refused: %v", err)
	}

	// A gap forward is legitimate: uplink messages can be lost, and the sequence
	// number they carry is what the receiver follows (TS 24.301 §4.4.3.1).
	if err := u.Commit(MakeCount(4, 12)); err != nil {
		t.Errorf("a count after a lost message was refused: %v", err)
	}

	// A count below the window is a replay: Estimate places that sequence number
	// after a wrap, so this is not the count it would have produced.
	if err := u.Commit(MakeCount(4, 5)); err == nil {
		t.Error("a replayed count was accepted")
	}

	// Nor may a caller commit an overflow the estimate never named.
	if err := u.Commit(MakeCount(9, 13)); err == nil {
		t.Error("a count from another overflow was accepted")
	}

	if u.LastAccepted() != MakeCount(4, 12) {
		t.Errorf("LastAccepted() = %#06x after refusals, want the last good commit", uint32(u.LastAccepted()))
	}
}

// mustEstimate is Estimate for a counter the test knows is not exhausted.
func mustEstimate(t *testing.T, u UplinkCounter, recvSeq uint8) Count {
	t.Helper()

	count, err := u.Estimate(recvSeq)
	if err != nil {
		t.Fatalf("Estimate(%#02x): %v", recvSeq, err)
	}

	return count
}

// TestUplinkCounterFailsClosedAtMax pins the receive side to the same rule the
// sender keeps: a NAS COUNT is used once under a key. Wrapping to zero would
// verify a replay of the first message and reuse its keystream (TS 33.401 §6.5,
// TS 33.501 §6.4.3.1), so the counter refuses to estimate past the maximum and
// says so, leaving the caller to release the connection or rekey.
func TestUplinkCounterFailsClosedAtMax(t *testing.T) {
	u := NewUplinkCounter(MakeCount(0xFFFF, 0xFE))

	last := mustEstimate(t, u, 0xFF)
	if last != MakeCount(0xFFFF, 0xFF) {
		t.Fatalf("estimate before the maximum = %#06x", uint32(last))
	}

	mustCommit(t, &u, last)

	if !u.Exhausted() {
		t.Fatal("committing the maximum count left the counter usable")
	}

	if _, err := u.Estimate(0x00); !errors.Is(err, ErrCountExhausted) {
		t.Errorf("Estimate past the maximum = %v, want ErrCountExhausted", err)
	}

	if err := u.Commit(MakeCount(0, 0)); !errors.Is(err, ErrCountExhausted) {
		t.Errorf("Commit past the maximum = %v, want ErrCountExhausted", err)
	}
}

func TestUplinkCounterRestoredAtMaxFailsClosed(t *testing.T) {
	u := NewUplinkCounter(MakeCount(0xFFFF, 0xFF))

	if !u.Exhausted() {
		t.Error("a counter restored at the maximum reports itself usable, so the context survives a relocation with a spent uplink NAS COUNT")
	}

	if _, err := u.Estimate(0x00); !errors.Is(err, ErrCountExhausted) {
		t.Errorf("Estimate on a counter restored at the maximum = %v, want ErrCountExhausted: wrapping to zero re-accepts the first message of the context under the same key", err)
	}

	if err := u.Commit(MakeCount(0, 0)); !errors.Is(err, ErrCountExhausted) {
		t.Errorf("Commit on a counter restored at the maximum = %v, want ErrCountExhausted", err)
	}
}
