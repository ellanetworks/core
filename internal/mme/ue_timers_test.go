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

// TestMobileReachableDerivesFromT3412 pins the mobile reachable timer to the
// periodic-TAU timer + 4 min (TS 24.301 §5.3.5), so the two cannot drift if
// T3412PeriodicTAU changes — the same value the Attach Accept advertises.
func TestMobileReachableDerivesFromT3412(t *testing.T) {
	m := newTestMME(t)

	if got, want := m.mobileReachableTime, T3412PeriodicTAU+4*time.Minute; got != want {
		t.Fatalf("mobileReachableTime = %v, want T3412PeriodicTAU + 4min = %v", got, want)
	}
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

	// The implicit detach transitions the EMM state under the registry lock.
	eventually(t, time.Second, func() bool {
		return ue.EMMState() == EMMDeregistered
	})

	// The EMM context is retained as a Deregistered husk keeping its native security
	// context, so a later re-attach with the native GUTI can reuse it (skip auth),
	// mirroring the AMF; it is not removed.
	if _, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok {
		t.Fatal("implicit detach must retain the UE context (husk) for native-context reuse")
	}

	if !ue.Secured() {
		t.Fatal("retained husk must keep its native security context")
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
	establishResumeForTest(m, ue, &captureConn{}, 9)

	time.Sleep(100 * time.Millisecond)

	if _, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); !ok {
		t.Fatal("UE implicitly detached despite reconnecting")
	}

	if m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session released despite reconnecting")
	}
}

// TestUEContextReleaseCompleteArmsMobileReachable confirms the supervision is
