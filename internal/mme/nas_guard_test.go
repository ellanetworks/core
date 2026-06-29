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

	m.ArmNASGuard(ue, "Authentication Request", []byte{0x07, 0x52})

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

	m.ArmNASGuardAbortOnly(ue, "Deactivate EPS Bearer Context Request", []byte{0x07, 0xc9}, func() {
		finalized <- struct{}{}
	})

	select {
	case <-finalized:
	case <-time.After(time.Second):
		t.Fatal("abort-only finalizer not run after retransmissions exhausted")
	}

	if ue.S1.releasing {
		t.Fatal("abort-only guard released the UE; expected it to stay connected")
	}

	// Two retransmissions, and no UE Context Release Command.
	if got := cc.count(); got != 2 {
		t.Fatalf("sent %d messages, want 2 retransmissions and no release", got)
	}
}

// TestESMGuardUsesESMTimeout confirms the ESM bearer-procedure guard (T3486
// modify, T3495 deactivate) fires at the ESM interval, not the common-procedure
// interval. The common timeout is set long so that only the ESM interval can
// drive the retransmissions within the test window.
func TestESMGuardUsesESMTimeout(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardTimeout = 10 * time.Second
	m.esmGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 2

	ue, cc := securedUE(t, m)

	p := &PdnConnection{Ebi: 5}

	finalized := make(chan struct{}, 1)

	m.ArmESMGuardAbortOnly(ue, p, "Modify EPS Bearer Context Request", []byte{0x07, 0xc9}, func() {
		finalized <- struct{}{}
	})

	select {
	case <-finalized:
	case <-time.After(time.Second):
		t.Fatal("ESM guard did not fire at esmGuardTimeout; likely using the common-procedure timeout")
	}

	if got := cc.count(); got != 2 {
		t.Fatalf("sent %d messages, want 2 ESM retransmissions", got)
	}
}

// TestPerBearerESMGuardsAreIndependent is the property that fixes the verified
// single-guard gap: each bearer's ESM procedure has its own guard, so a procedure
// outstanding on one bearer (or on EMM) does not cancel another's retransmissions.
// With a single shared guard, arming the second would cancel the first and its
// finalizer would never run.
func TestPerBearerESMGuardsAreIndependent(t *testing.T) {
	m := newTestMME(t)
	m.esmGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 1

	ue, _ := securedUE(t, m)

	p1 := &PdnConnection{Ebi: 5}
	p2 := &PdnConnection{Ebi: 6}

	a1 := make(chan struct{}, 1)
	a2 := make(chan struct{}, 1)

	m.ArmESMGuardAbortOnly(ue, p1, "Modify EPS Bearer Context Request", []byte{0x07, 0xc9}, func() { a1 <- struct{}{} })
	m.ArmESMGuardAbortOnly(ue, p2, "Deactivate EPS Bearer Context Request", []byte{0x07, 0xcd}, func() { a2 <- struct{}{} })

	for i, ch := range []chan struct{}{a1, a2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("ESM guard %d finalizer never ran: a concurrent bearer's guard cancelled it", i+1)
		}
	}
}

// TestNASGuardStoppedByResponse confirms a UE response cancels the guard before
// it can retransmit or release.
func TestNASGuardStoppedByResponse(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 2

	ue, cc := securedUE(t, m)

	m.ArmNASGuard(ue, "Authentication Request", []byte{0x07, 0x52})
	m.StopNASGuard(ue)

	// The guard is cancelled, so after the timeout window nothing mutates the UE.
	time.Sleep(50 * time.Millisecond)

	if ue.S1.releasing {
		t.Fatal("UE released despite the guarded response arriving")
	}

	if ue.S1.nasGuard.Active() {
		t.Fatal("NAS guard still armed after the response")
	}

	if got := cc.count(); got != 0 {
		t.Fatalf("sent %d messages after a stopped guard, want 0", got)
	}
}
