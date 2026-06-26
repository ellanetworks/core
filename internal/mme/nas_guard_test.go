// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
	"time"
)

// TestNASGuardRetransmitsThenReleases confirms an unanswered guarded procedure
// is retransmitted up to the limit and then aborted by releasing the UE
// (TS 24.301 §10.2).
func TestNASGuardRetransmitsThenReleases(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 2

	ue, cc := securedUE(t, m)

	m.armNASGuard(ue, "Authentication Request", []byte{0x07, 0x52})

	// Two retransmissions plus the UE Context Release Command. Wait for all three
	// sends rather than the releasing flag, which releaseUEContext sets just before
	// it sends the release command.
	eventually(t, time.Second, func() bool {
		return cc.count() >= 3
	})
}

// TestNASGuardAbortOnlyRunsFinalizer confirms an abort-only guard, on exhausting
// its retransmissions, runs its finalizer and leaves the UE connected rather
// than releasing the context (TS 24.301 §6.4.2.5, §6.4.4.5).
func TestNASGuardAbortOnlyRunsFinalizer(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 2

	ue, cc := securedUE(t, m)

	finalized := make(chan struct{}, 1)

	m.armNASGuardAbortOnly(ue, "Deactivate EPS Bearer Context Request", []byte{0x07, 0xc9}, func() {
		finalized <- struct{}{}
	})

	select {
	case <-finalized:
	case <-time.After(time.Second):
		t.Fatal("abort-only finalizer not run after retransmissions exhausted")
	}

	if ue.s1.releasing {
		t.Fatal("abort-only guard released the UE; expected it to stay connected")
	}

	// Two retransmissions, and no UE Context Release Command.
	if got := cc.count(); got != 2 {
		t.Fatalf("sent %d messages, want 2 retransmissions and no release", got)
	}
}

// TestNASGuardStoppedByResponse confirms a UE response cancels the guard before
// it can retransmit or release.
func TestNASGuardStoppedByResponse(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 2

	ue, cc := securedUE(t, m)

	m.armNASGuard(ue, "Authentication Request", []byte{0x07, 0x52})
	m.stopNASGuard(ue)

	// The guard is cancelled, so after the timeout window nothing mutates the UE.
	time.Sleep(50 * time.Millisecond)

	if ue.s1.releasing {
		t.Fatal("UE released despite the guarded response arriving")
	}

	if ue.s1.nasGuardTimer != nil {
		t.Fatal("NAS guard still armed after the response")
	}

	if got := cc.count(); got != 0 {
		t.Fatalf("sent %d messages after a stopped guard, want 0", got)
	}
}
