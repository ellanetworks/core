// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package guard

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestArmRetransmitsThenAborts(t *testing.T) {
	var retransmits atomic.Int32

	aborted := make(chan struct{})

	var g Guard
	g.Arm(15*time.Millisecond, 2,
		func(int32) { retransmits.Add(1) },
		func() { close(aborted) },
	)

	select {
	case <-aborted:
	case <-time.After(2 * time.Second):
		t.Fatal("onAbort did not fire within timeout")
	}

	if got := retransmits.Load(); got != 2 {
		t.Errorf("retransmits = %d, want 2 (one per interval up to the limit)", got)
	}

	if g.Active() {
		t.Error("guard still active after abort")
	}
}

func TestStopPreventsRetransmitAndAbort(t *testing.T) {
	var retransmits, aborts atomic.Int32

	var g Guard
	g.Arm(20*time.Millisecond, 5,
		func(int32) { retransmits.Add(1) },
		func() { aborts.Add(1) },
	)

	g.Stop()

	if g.Active() {
		t.Error("guard active after Stop")
	}

	time.Sleep(120 * time.Millisecond)

	if got := retransmits.Load(); got != 0 {
		t.Errorf("retransmits = %d, want 0 after immediate Stop", got)
	}

	if got := aborts.Load(); got != 0 {
		t.Errorf("aborts = %d, want 0 after Stop", got)
	}
}

func TestRearmInvalidatesPreviousCallbacks(t *testing.T) {
	var firstFired, secondAborted atomic.Int32

	abortedSecond := make(chan struct{})

	var g Guard
	g.Arm(15*time.Millisecond, 10,
		func(int32) { firstFired.Add(1) },
		func() { firstFired.Add(1) },
	)

	// Re-arm before the first guard could exhaust; the first generation's
	// callbacks must never run again.
	g.Arm(15*time.Millisecond, 1,
		func(int32) {},
		func() { secondAborted.Add(1); close(abortedSecond) },
	)

	before := firstFired.Load()

	select {
	case <-abortedSecond:
	case <-time.After(2 * time.Second):
		t.Fatal("re-armed guard did not abort within timeout")
	}

	if got := firstFired.Load() - before; got != 0 {
		t.Errorf("first-generation callbacks fired %d times after re-arm, want 0", got)
	}
}

func TestArmOnceFiresOnce(t *testing.T) {
	var fired atomic.Int32

	done := make(chan struct{})

	var g Guard
	g.ArmOnce(15*time.Millisecond, func() { fired.Add(1); close(done) })

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onFire did not run within timeout")
	}

	// Give any erroneous repeat interval time to elapse.
	time.Sleep(60 * time.Millisecond)

	if got := fired.Load(); got != 1 {
		t.Errorf("onFire ran %d times, want exactly 1", got)
	}

	if g.Active() {
		t.Error("guard still active after one-shot fired")
	}
}

func TestArmOnceStopPreventsFire(t *testing.T) {
	var fired atomic.Int32

	var g Guard
	g.ArmOnce(30*time.Millisecond, func() { fired.Add(1) })
	g.Stop()

	time.Sleep(120 * time.Millisecond)

	if got := fired.Load(); got != 0 {
		t.Errorf("onFire ran %d times after Stop, want 0", got)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	var g Guard

	g.Arm(time.Second, 1, func(int32) {}, func() {})
	g.Stop()
	g.Stop() // must not panic
}

func TestConcurrentArmStop(t *testing.T) {
	var g Guard

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			g.Arm(5*time.Millisecond, 3, func(int32) {}, func() {})
			g.Stop()
		}()
	}

	wg.Wait()
	g.Stop()
}
