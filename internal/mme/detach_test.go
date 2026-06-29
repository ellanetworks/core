// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"
)

func TestDetachSubscriberUnansweredReleases(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 2

	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	// Initial Detach Request + 2 retransmissions + the UE Context Release Command.
	eventually(t, time.Second, func() bool {
		return cc.count() >= 4
	})

	if !ue.S1.releasing {
		t.Fatal("UE not released after an unanswered network-initiated detach")
	}
}

func TestDetachSubscriberNotAttachedNoop(t *testing.T) {
	m := newTestMME(t)
	// No UE attached for this IMSI: must be a no-op (no panic, nothing sent).
	m.DetachSubscriber(context.Background(), "001010000000999")
}

// TestForgedMessageIgnoredForSecuredUE checks that once the secure exchange of
// NAS messages is established, a message that fails the integrity check (here a
// forged DETACH REQUEST) is discarded, not processed. TS 24.301 §4.4.4.3
// recovery applies only before that point (no usable context in the network),
// so an attacker cannot tear down an authenticated UE with an unverifiable
// message.
func securedUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	kasme := make([]byte, 32)
	for i := range kasme {
		kasme[i] = byte(i + 1)
	}

	ue.kasme = kasme
	ue.eea, ue.eia = 2, 2

	var err error
	if ue.knasEnc, err = DeriveKNASEnc(kasme, 2); err != nil {
		t.Fatal(err)
	}

	if ue.knasInt, err = DeriveKNASInt(kasme, 2); err != nil {
		t.Fatal(err)
	}

	ue.secured = true
	ue.S1.secureExchangeEstablished = true
	ue.emmState.store(EMMRegistered)
	registerTestUE(m, ue, testSubscriber.IMSI)

	return ue, cc
}

// registerTestUE sets a UE's IMSI and indexes it in the persistent registry, as a
// completed attach would. Re-registering a UE under a new IMSI moves its index.
func registerTestUE(m *MME, ue *UeContext, imsi string) {
	m.mu.Lock()
	if ue.imsi != "" && m.ues[ue.imsi] == ue {
		delete(m.ues, ue.imsi)
	}

	ue.imsi = imsi
	m.ues[imsi] = ue
	m.mu.Unlock()
}

// TestDetachSubscriberIdleReleasesLocally checks that deleting a subscriber whose
// UE is in ECM-IDLE releases its sessions and removes the context locally, without
// dereferencing the freed S1 connection.
func TestDetachSubscriberIdleReleasesLocally(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"
	m.FreeS1Conn(ue) // ECM-IDLE: no S1 connection

	m.DetachSubscriber(context.Background(), ue.imsi)

	if _, ok := m.LookupUeByIMSI(ue.imsi); ok {
		t.Fatal("idle UE context not removed on subscriber deletion")
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released on subscriber deletion")
	}
}

// TestReleaseUEContextIdleNoPanic checks releaseUEContext on a UE whose connection
// was freed in the gap before it took the lock returns without dereferencing nil.
func TestReleaseUEContextIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	m.FreeS1Conn(ue)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)
}
