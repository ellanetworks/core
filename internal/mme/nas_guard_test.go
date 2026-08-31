// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §10.2
func TestNASGuardRetransmitsThenReleases(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 5 * time.Millisecond
	m.nasGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	ue.Conn().ArmNASGuard("Authentication Request", []byte{0x07, 0x52}, eps.SHTIntegrityProtectedCiphered)

	eventually(t, time.Second, func() bool {
		return cc.count() >= 3
	})
}

// TS 24.301 §6.4.2.5
func TestNASGuardAbortOnlyRunsFinalizer(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 5 * time.Millisecond
	m.nasGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	finalized := make(chan struct{}, 1)

	ue.Conn().ArmNASGuardAbortOnly("Deactivate EPS Bearer Context Request", []byte{0x07, 0xc9}, eps.SHTIntegrityProtectedCiphered, func() {
		finalized <- struct{}{}
	})

	select {
	case <-finalized:
	case <-time.After(time.Second):
		t.Fatal("abort-only finalizer not run after retransmissions exhausted")
	}

	if ue.ReleasingForTest() {
		t.Fatal("abort-only guard released the UE; expected it to stay connected")
	}

	if got := cc.count(); got != 2 {
		t.Fatalf("sent %d messages, want 2 retransmissions and no release", got)
	}
}

func TestESMGuardUsesESMTimeout(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 10 * time.Second
	m.esmGuardCfg.ExpireTime = 5 * time.Millisecond
	m.esmGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	p := &PdnConnection{Ebi: 5}

	finalized := make(chan struct{}, 1)

	m.ArmESMGuardAbortOnly(ue, p, "Modify EPS Bearer Context Request", []byte{0x07, 0xc9}, eps.SHTIntegrityProtectedCiphered, func() {
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

func TestPerBearerESMGuardsAreIndependent(t *testing.T) {
	m := newTestMME(t)
	m.esmGuardCfg.ExpireTime = 5 * time.Millisecond
	m.esmGuardCfg.MaxRetryTimes = 1

	ue, _ := securedUE(t, m)

	p1 := &PdnConnection{Ebi: 5}
	p2 := &PdnConnection{Ebi: 6}

	a1 := make(chan struct{}, 1)
	a2 := make(chan struct{}, 1)

	m.ArmESMGuardAbortOnly(ue, p1, "Modify EPS Bearer Context Request", []byte{0x07, 0xc9}, eps.SHTIntegrityProtectedCiphered, func() { a1 <- struct{}{} })
	m.ArmESMGuardAbortOnly(ue, p2, "Deactivate EPS Bearer Context Request", []byte{0x07, 0xcd}, eps.SHTIntegrityProtectedCiphered, func() { a2 <- struct{}{} })

	for i, ch := range []chan struct{}{a1, a2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("ESM guard %d finalizer never ran: a concurrent bearer's guard cancelled it", i+1)
		}
	}
}

func TestNASGuardStoppedByResponse(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 5 * time.Millisecond
	m.nasGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	ue.Conn().ArmNASGuard("Authentication Request", []byte{0x07, 0x52}, eps.SHTIntegrityProtectedCiphered)
	ue.Conn().StopNASGuard()

	time.Sleep(50 * time.Millisecond)

	if ue.ReleasingForTest() {
		t.Fatal("UE released despite the guarded response arriving")
	}

	if ue.Conn().nasGuard.Active() {
		t.Fatal("NAS guard still armed after the response")
	}

	if got := cc.count(); got != 0 {
		t.Fatalf("sent %d messages after a stopped guard, want 0", got)
	}
}
