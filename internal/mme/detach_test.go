// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
)

func TestDetachSubscriberUnansweredReleases(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 5 * time.Millisecond
	m.nasGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	// Initial Detach Request + 2 retransmissions + the UE Context Release Command.
	eventually(t, time.Second, func() bool {
		return cc.count() >= 4
	})

	if !ue.Conn().releasing {
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
	ue.cipheringAlg, ue.integrityAlg = 2, 2

	var err error
	if ue.knasEnc, err = DeriveKNASEnc(kasme, 2); err != nil {
		t.Fatal(err)
	}

	if ue.knasInt, err = DeriveKNASInt(kasme, 2); err != nil {
		t.Fatal(err)
	}

	ue.secured = true
	ue.Conn().secureExchangeEstablished = true
	ue.ForceStateForTest(EMMRegistered)
	registerTestUE(m, ue, testSubscriber.IMSI)

	return ue, cc
}

// registerTestUE sets a UE's IMSI and indexes it in the persistent registry, as a
// completed attach would. Re-registering a UE under a new IMSI moves its index.
func registerTestUE(m *MME, ue *UeContext, imsi string) {
	m.mu.Lock()
	if ue.supi.IsIMSI() && m.UEs[ue.supi] == ue {
		delete(m.UEs, ue.supi)
	}

	ue.supi, _ = etsi.NewSUPIFromIMSI(imsi)
	m.UEs[ue.supi] = ue
	m.mu.Unlock()
}

// TestDetachSubscriberIdleReleasesLocally checks that deleting a subscriber whose
// UE is in ECM-IDLE releases its sessions and removes the context locally, without
// dereferencing the freed S1 connection.
func TestDetachSubscriberIdleReleasesLocally(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"
	m.FreeUeConn(ue) // ECM-IDLE: no S1 connection

	m.DetachSubscriber(context.Background(), ue.imsiOrEmpty())

	if _, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); ok {
		t.Fatal("idle UE context not removed on subscriber deletion")
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released on subscriber deletion")
	}
}

// TestDetachSubscriberConnectedUnsecuredReleasesLocally checks that deleting a
// subscriber whose UE is connected but has no security context (e.g. mid-attach
// before security mode) removes the context and releases its sessions locally,
// rather than leaving the deleted subscriber connected because a protected DETACH
// REQUEST could not be built (TS 24.301 §5.5.2.3.1 local detach). Mirrors the AMF.
func TestDetachSubscriberConnectedUnsecuredReleasesLocally(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.secured = false // connected, but no NAS security context
	ue.ForceStateForTest(EMMRegistrationInitiated)
	registerTestUE(m, ue, testSubscriber.IMSI)
	testPDN(ue).Apn = "internet"

	if !m.UeConnected(ue) {
		t.Fatal("test precondition: UE must be connected")
	}

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	if _, ok := m.LookupUeByIMSI(testSubscriber.IMSI); ok {
		t.Fatal("connected-but-unsecured UE context not removed on subscriber deletion")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no downlink for a local detach (no keys to protect one), got %d", cc.count())
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released on local detach")
	}
}

// TestReleaseUEContextIdleNoPanic checks releaseUEContext on a UE whose connection
// was freed in the gap before it took the lock returns without dereferencing nil.
func TestReleaseUEContextIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	m.FreeUeConn(ue)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)
}
