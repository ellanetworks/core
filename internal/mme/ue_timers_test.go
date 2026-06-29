// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
	"time"
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

	m.StartMobileReachable(ue)

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

	m.StartMobileReachable(ue)
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
