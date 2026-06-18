// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package timer_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/util/timer"
)

func TestNewTimerIsActive(t *testing.T) {
	tm := timer.New(1*time.Hour, 1, func(_ int32) {}, func() {})
	defer tm.Stop()

	if !tm.IsActive() {
		t.Fatal("expected new timer to be active, got inactive")
	}
}

func TestTimerIsActiveAfterStop(t *testing.T) {
	tm := timer.New(1*time.Hour, 1, func(_ int32) {}, func() {})
	tm.Stop()

	if tm.IsActive() {
		t.Fatal("expected stopped timer to be inactive, got active")
	}

	tm.Stop() // safe to call again
}

func TestTimerRetransmitsThenCancels(t *testing.T) {
	var expiries int32

	cancelled := make(chan struct{})
	tm := timer.New(10*time.Millisecond, 3,
		func(_ int32) { atomic.AddInt32(&expiries, 1) },
		func() { close(cancelled) })

	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("timer did not cancel within timeout")
	}

	if got := atomic.LoadInt32(&expiries); got != 3 {
		t.Errorf("expiredFunc calls = %d, want 3 (cancel on the 4th expiry)", got)
	}

	if tm.IsActive() {
		t.Error("expected cancelled timer to be inactive")
	}
}

func TestTimerStopPreventsCallbacks(t *testing.T) {
	var fired int32

	tm := timer.New(10*time.Millisecond, 5, func(_ int32) { atomic.AddInt32(&fired, 1) }, func() {})
	tm.Stop()

	time.Sleep(60 * time.Millisecond)

	if got := atomic.LoadInt32(&fired); got != 0 {
		t.Errorf("expiredFunc fired %d times after Stop, want 0", got)
	}
}
