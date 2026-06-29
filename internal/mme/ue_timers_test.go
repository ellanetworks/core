// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/s1ap"
)

func eventually(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("condition not met within deadline")
}

// TestMobileReachableEscalatesToImplicitDetach drives the full idle-supervision
// escalation (TS 24.301 §5.3.5): the mobile reachable timer expires, the
// implicit detach timer expires, and the UE is released locally.
func TestMobileReachableEscalatesToImplicitDetach(t *testing.T) {
	m := newTestMME(t)
	m.mobileReachableTime = 10 * time.Millisecond
	m.implicitDetachTime = 10 * time.Millisecond

	ue := idleRegisteredUE(t, m)
	testPDN(ue).Apn = "internet" // so the implicit detach releases the EPS session

	m.startMobileReachable(ue)

	eventually(t, time.Second, func() bool {
		_, ok := m.LookupUeByIMSI(ue.imsi)
		return !ok
	})

	if ue.emmState.load() != EMMDeregistered {
		t.Fatalf("emmState = %v, want EMMDeregistered after implicit detach", ue.emmState.load())
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released on implicit detach")
	}
}

// TestReconnectStopsIdleTimers confirms a UE that re-establishes a NAS
// connection before the timers expire is not implicitly detached.
func TestReconnectStopsIdleTimers(t *testing.T) {
	m := newTestMME(t)
	m.mobileReachableTime = 20 * time.Millisecond
	m.implicitDetachTime = 20 * time.Millisecond

	ue := idleRegisteredUE(t, m)
	testPDN(ue).Apn = "internet"

	m.startMobileReachable(ue)
	m.EstablishS1Connection(ue, &captureConn{}, 9)

	time.Sleep(100 * time.Millisecond)

	if _, ok := m.LookupUeByIMSI(ue.imsi); !ok {
		t.Fatal("UE implicitly detached despite reconnecting")
	}

	if m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session released despite reconnecting")
	}
}

// TestUEContextReleaseCompleteArmsMobileReachable confirms the supervision is
// armed when a registered UE moves to ECM-IDLE on an S1 release.
func TestUEContextReleaseCompleteArmsMobileReachable(t *testing.T) {
	m := newTestMME(t)

	ue, cc := securedUE(t, m) // connected; the release moves it to ECM-IDLE

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.S1.MMEUES1APID, ENBUES1APID: 7}

	b, err := complete.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	cpdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	m.HandleUEContextReleaseComplete(cc, cpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.Connected() {
		t.Fatal("UE still connected after S1 release")
	}

	if ue.mobileReachableTimer == nil {
		t.Fatal("mobile reachable timer not armed when UE moved to ECM-IDLE")
	}

	m.RemoveUe(ue) // stop the default-duration timer
}
