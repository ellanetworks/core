// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/nas"
)

func TestDetachSubscriberUnansweredReleases(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 5 * time.Millisecond
	m.nasGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	eventually(t, time.Second, func() bool {
		return cc.count() >= 4
	})

	if !ue.ReleasingForTest() {
		t.Fatal("UE not released after an unanswered network-initiated detach")
	}
}

func TestDetachSubscriberNotAttachedNoop(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), "001010000000999")

	if cc.count() != 0 {
		t.Fatalf("detaching an unattached IMSI sent %d messages", cc.count())
	}

	if ue.ReleasingForTest() {
		t.Fatal("detaching an unattached IMSI released the wrong subscriber")
	}

	if _, ok := m.LookupUeByIMSI(testSubscriber.IMSI); !ok {
		t.Fatal("detaching an unattached IMSI dropped the attached subscriber")
	}
}

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

	if ue.knasEnc, err = epskeys.DeriveKNASEnc(kasme, 2); err != nil {
		t.Fatal(err)
	}

	if ue.knasInt, err = epskeys.DeriveKNASInt(kasme, 2); err != nil {
		t.Fatal(err)
	}

	sc, err := ue.installSecurityContextLocked()
	if err != nil {
		t.Fatal(err)
	}

	ue.downlink().Install(sc, nas.DownlinkCounter{})

	ue.secured = true
	ue.Conn().secureExchangeEstablished = true
	ue.ForceStateForTest(EMMRegistered)
	ue.SetAccess(Access{Allow4G: true, Allow5G: true})
	registerTestUE(m, ue, testSubscriber.IMSI)

	return ue, cc
}

func registerTestUE(m *MME, ue *UeContext, imsi string) {
	m.mu.Lock()
	if ue.supi.IsIMSI() && m.UEs[ue.supi] == ue {
		delete(m.UEs, ue.supi)
	}

	ue.supi, _ = etsi.NewSUPIFromIMSI(imsi)
	m.UEs[ue.supi] = ue
	m.mu.Unlock()
}

func TestDetachSubscriberIdleReleasesLocally(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"
	m.FreeUeConn(ue)

	m.DetachSubscriber(context.Background(), ue.imsiOrEmpty())

	if _, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); ok {
		t.Fatal("idle UE context not removed on subscriber deletion")
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released on subscriber deletion")
	}
}

// TS 24.301 §5.5.2.3.1
func TestDetachSubscriberConnectedUnsecuredReleasesLocally(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.secured = false
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

func TestReleaseUEContextIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	m.FreeUeConn(ue)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	if cc.count() != 0 {
		t.Fatalf("a UE with no connection cannot be sent a Release Command, got %d messages", cc.count())
	}

	if ue.Conn() != nil {
		t.Fatal("the freed connection was resurrected")
	}
}
