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

// TS 24.301 §5.3.5
func TestMobileReachableDerivesFromT3412(t *testing.T) {
	m := newTestMME(t)

	if got, want := m.mobileReachableTime, T3412PeriodicTAU+4*time.Minute; got != want {
		t.Fatalf("mobileReachableTime = %v, want T3412PeriodicTAU + 4min = %v", got, want)
	}
}

// TS 24.301 §5.3.5
func TestMobileReachableEscalatesToImplicitDetach(t *testing.T) {
	m := newTestMME(t)
	m.mobileReachableTime = 10 * time.Millisecond
	m.implicitDetachTime = 10 * time.Millisecond

	ue := idleRegisteredUE(t, m)
	testPDN(ue).Apn = "internet"

	m.StartMobileReachable(ue)

	eventually(t, time.Second, func() bool {
		return ue.EMMState() == EMMDeregistered
	})

	if _, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok {
		t.Fatal("implicit detach must retain the UE context (husk) for native-context reuse")
	}

	if !ue.Secured() {
		t.Fatal("retained husk must keep its native security context")
	}
}

func TestReconnectStopsIdleTimers(t *testing.T) {
	m := newTestMME(t)
	m.mobileReachableTime = 20 * time.Millisecond
	m.implicitDetachTime = 20 * time.Millisecond

	ue := idleRegisteredUE(t, m)
	testPDN(ue).Apn = "internet"

	m.StartMobileReachable(ue)
	establishResumeForTest(m, ue, &captureConn{}, 9)

	time.Sleep(100 * time.Millisecond)

	if _, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok {
		t.Fatal("UE implicitly detached despite reconnecting")
	}

	if m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session released despite reconnecting")
	}
}
