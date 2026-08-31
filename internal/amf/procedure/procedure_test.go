// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package procedure_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"go.uber.org/zap"
)

func newTestRegistry() *procedure.Registry {
	return procedure.NewRegistry(zap.NewNop())
}

func waitUntil(t *testing.T, cond func() bool, what string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(time.Millisecond)
	}
}

func pausedTeardown(t *testing.T, r *procedure.Registry, d procedure.Disposition) (finish func()) {
	t.Helper()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	started, release := make(chan struct{}), make(chan struct{})
	announce := sync.OnceFunc(func() { close(started) })

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Millisecond),
		func(context.Context) (procedure.Disposition, error) {
			announce()
			<-release

			return d, nil
		})
	if err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	<-started

	return sync.OnceFunc(func() { close(release) })
}

func TestTeardownHoldsTheSlotUntilItReturns(t *testing.T) {
	r := newTestRegistry()

	finish := pausedTeardown(t, r, procedure.Release)
	defer finish()

	if !r.Active(procedure.N2Handover) {
		t.Fatal("the procedure being torn down reported itself inactive")
	}

	if err := r.Begin(procedure.SecurityMode); !errors.Is(err, procedure.ErrConflict) {
		t.Fatalf("a conflicting procedure began mid-teardown: Begin returned %v", err)
	}

	finish()

	waitUntil(t, func() bool { return !r.Active(procedure.N2Handover) }, "the teardown to release the slot")

	if err := r.Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("the slot was not free after the teardown: %v", err)
	}
}

func TestTheFreedSlotHappensAfterTheTeardownsLastStep(t *testing.T) {
	r := newTestRegistry()

	var lastStepDone atomic.Bool

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Millisecond),
		func(context.Context) (procedure.Disposition, error) {
			time.Sleep(2 * time.Millisecond)
			lastStepDone.Store(true)

			return procedure.Release, nil
		})
	if err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	waitUntil(t, func() bool { return !r.Active(procedure.N2Handover) }, "the procedure to be torn down")

	if !lastStepDone.Load() {
		t.Fatal("the slot went free before the teardown's last step")
	}
}

func TestRetainKeepsTheProcedureActivePastItsDeadline(t *testing.T) {
	r := newTestRegistry()

	finish := pausedTeardown(t, r, procedure.Retain)
	finish()

	time.Sleep(20 * time.Millisecond)

	if !r.Active(procedure.N2Handover) {
		t.Fatal("a retained procedure lost its slot to its own deadline")
	}

	if err := r.Begin(procedure.PathSwitch); !errors.Is(err, procedure.ErrConflict) {
		t.Fatalf("a path switch began while a retained handover held the key chain: Begin returned %v", err)
	}

	waitUntil(t, func() bool {
		r.End(procedure.N2Handover)

		return !r.Active(procedure.N2Handover)
	}, "End to release the retained procedure")
}

func TestRetainEndsSupervisionRatherThanRespinningIt(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	var calls atomic.Int32

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Millisecond),
		func(context.Context) (procedure.Disposition, error) {
			calls.Add(1)

			return procedure.Retain, nil
		})
	if err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	waitUntil(t, func() bool { return calls.Load() > 0 }, "the deadline to expire")

	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Errorf("the cancel callback ran %d times, want 1: the deadline re-arms itself for as long as the commit is stuck", got)
	}

	if !r.Active(procedure.N2Handover) {
		t.Error("the retained procedure lost its slot, so the next procedure can rekey the UE underneath the commit")
	}
}

func TestCancelDoesNotOverrideRetain(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	var calls atomic.Int32

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Hour),
		func(context.Context) (procedure.Disposition, error) {
			calls.Add(1)

			return procedure.Retain, nil
		})
	if err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	if err := r.Cancel(context.Background(), procedure.N2Handover); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("the cancel callback ran %d times, want 1", got)
	}

	if !r.Active(procedure.N2Handover) {
		t.Error("Cancel freed the slot against the callback's Retain, so the next procedure can rekey the UE underneath the commit")
	}

	r.End(procedure.N2Handover)

	if r.Active(procedure.N2Handover) {
		t.Error("End did not free a retained procedure, so nothing ever recovers the key chain")
	}
}

func TestEndDuringTeardownDoesNotFreeTheSlotEarly(t *testing.T) {
	r := newTestRegistry()

	finish := pausedTeardown(t, r, procedure.Release)
	defer finish()

	r.End(procedure.N2Handover)

	if !r.Active(procedure.N2Handover) {
		t.Fatal("End freed the slot while the teardown was still running")
	}

	finish()

	waitUntil(t, func() bool { return !r.Active(procedure.N2Handover) }, "the teardown to release the slot")
}

func TestEndDuringTeardownOutranksRetain(t *testing.T) {
	r := newTestRegistry()

	finish := pausedTeardown(t, r, procedure.Retain)
	defer finish()

	r.End(procedure.N2Handover)
	finish()

	waitUntil(t, func() bool { return !r.Active(procedure.N2Handover) },
		"a retained-but-ended procedure to release the slot")
}

func TestCancelAndSuperviseAreRejectedDuringTeardown(t *testing.T) {
	r := newTestRegistry()

	finish := pausedTeardown(t, r, procedure.Release)
	defer finish()

	if err := r.Cancel(context.Background(), procedure.N2Handover); !errors.Is(err, procedure.ErrSettling) {
		t.Errorf("Cancel during a teardown returned %v, want ErrSettling", err)
	}

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Minute),
		func(context.Context) (procedure.Disposition, error) { return procedure.Release, nil })
	if !errors.Is(err, procedure.ErrSettling) {
		t.Errorf("Supervise during a teardown returned %v, want ErrSettling", err)
	}
}

func TestExplicitCancelAlsoHoldsTheSlot(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	started, release := make(chan struct{}), make(chan struct{})

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Minute),
		func(context.Context) (procedure.Disposition, error) {
			close(started)
			<-release

			return procedure.Release, nil
		})
	if err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	cancelled := make(chan error, 1)

	go func() { cancelled <- r.Cancel(context.Background(), procedure.N2Handover) }()

	<-started

	if err := r.Begin(procedure.SecurityMode); !errors.Is(err, procedure.ErrConflict) {
		t.Errorf("a conflicting procedure began mid-cancel: Begin returned %v", err)
	}

	close(release)

	if err := <-cancelled; err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("Cancel left the procedure active")
	}
}

func TestPanickingTeardownReleasesTheSlot(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Millisecond),
		func(context.Context) (procedure.Disposition, error) { panic("intentional panic") })
	if err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	waitUntil(t, func() bool { return !r.Active(procedure.N2Handover) }, "the panicking teardown to release the slot")
}

func TestBeginEndRoundTrip(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if !r.Active(procedure.SecurityMode) {
		t.Fatal("expected SecurityMode to be active")
	}

	r.End(procedure.SecurityMode)

	if r.Active(procedure.SecurityMode) {
		t.Fatal("expected SecurityMode to be inactive after End")
	}
}

func TestSameTypeConflict(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("first Begin failed: %v", err)
	}

	if err := r.Begin(procedure.N2Handover); !errors.Is(err, procedure.ErrAlreadyActive) {
		t.Fatalf("expected ErrAlreadyActive, got %v", err)
	}
}

// TS 33.501 §6.9.5
func TestKeyChainMutualExclusion(t *testing.T) {
	tests := []struct {
		first  procedure.Type
		second procedure.Type
		desc   string
	}{
		{procedure.SecurityMode, procedure.N2Handover, "SMC blocks N2Handover"},
		{procedure.N2Handover, procedure.SecurityMode, "N2Handover blocks SMC"},
		{procedure.SecurityMode, procedure.PathSwitch, "SMC blocks PathSwitch"},
		{procedure.PathSwitch, procedure.SecurityMode, "PathSwitch blocks SMC"},
		{procedure.N2Handover, procedure.PathSwitch, "N2Handover blocks PathSwitch"},
		{procedure.PathSwitch, procedure.N2Handover, "PathSwitch blocks N2Handover"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			r := newTestRegistry()

			if err := r.Begin(tt.first); err != nil {
				t.Fatalf("first Begin(%s) failed: %v", tt.first, err)
			}

			if err := r.Begin(tt.second); !errors.Is(err, procedure.ErrConflict) {
				t.Fatalf("%s: expected ErrConflict, got %v", tt.desc, err)
			}
		})
	}
}

func TestSuperviseArmsDeadlineAfterBegin(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	var cancelled atomic.Bool

	err := r.Supervise(procedure.N2Handover, time.Now().Add(50*time.Millisecond),
		func(context.Context) (procedure.Disposition, error) {
			cancelled.Store(true)
			return procedure.Release, nil
		})
	if err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if !cancelled.Load() {
		t.Fatal("expected supervised cancel callback to fire on timeout")
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover removed after supervised timeout")
	}
}

func TestSuperviseTimerStoppedByEnd(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	var cancelled atomic.Bool

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(50*time.Millisecond),
		func(context.Context) (procedure.Disposition, error) {
			cancelled.Store(true)
			return procedure.Release, nil
		}); err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	r.End(procedure.N2Handover)

	time.Sleep(150 * time.Millisecond)

	if cancelled.Load() {
		t.Fatal("End must stop the supervision timer so the cancel never fires")
	}
}

func TestStaleSuperviseTimerDoesNotExpireRebegun(t *testing.T) {
	r := newTestRegistry()

	var fired atomic.Bool

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("first Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(40*time.Millisecond),
		func(context.Context) (procedure.Disposition, error) { fired.Store(true); return procedure.Release, nil }); err != nil {
		t.Fatalf("first Supervise failed: %v", err)
	}

	r.End(procedure.N2Handover)

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("second Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(400*time.Millisecond),
		func(context.Context) (procedure.Disposition, error) { fired.Store(true); return procedure.Release, nil }); err != nil {
		t.Fatalf("second Supervise failed: %v", err)
	}

	time.Sleep(120 * time.Millisecond)

	if fired.Load() {
		t.Fatal("stale supervision timer expired the re-begun procedure")
	}

	if !r.Active(procedure.N2Handover) {
		t.Fatal("re-begun N2Handover should still be active")
	}
}

func TestSuperviseNotActive(t *testing.T) {
	r := newTestRegistry()

	err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Minute),
		func(context.Context) (procedure.Disposition, error) { return procedure.Release, nil })
	if !errors.Is(err, procedure.ErrNotActive) {
		t.Fatalf("expected ErrNotActive, got %v", err)
	}
}

func TestCancelInvokesCallbackAndRemoves(t *testing.T) {
	r := newTestRegistry()

	var called atomic.Bool

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Minute),
		func(context.Context) (procedure.Disposition, error) {
			called.Store(true)
			return procedure.Release, nil
		}); err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	if err := r.Cancel(context.Background(), procedure.N2Handover); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	if !called.Load() {
		t.Fatal("expected Cancel to invoke the cancel callback")
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover removed after Cancel")
	}
}

func TestCancelCallbackPanicRecovery(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Minute),
		func(context.Context) (procedure.Disposition, error) { panic("intentional panic") }); err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	if err := r.Cancel(context.Background(), procedure.N2Handover); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover removed even after panic")
	}
}

func TestCancelCallbackErrorStillRemoves(t *testing.T) {
	r := newTestRegistry()

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(time.Minute),
		func(context.Context) (procedure.Disposition, error) {
			return procedure.Release, errors.New("cancel failed")
		}); err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	if err := r.Cancel(context.Background(), procedure.N2Handover); err != nil {
		t.Fatalf("Cancel should succeed even if callback errors: %v", err)
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover removed even after callback error")
	}
}

func TestConcurrentBeginConflict(t *testing.T) {
	r := newTestRegistry()

	const n = 50

	var (
		successes atomic.Int32
		wg        sync.WaitGroup
	)

	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()

			if err := r.Begin(procedure.N2Handover); err == nil {
				successes.Add(1)
			}
		}()
	}

	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("expected exactly 1 success, got %d", got)
	}
}

func TestCancelNotActive(t *testing.T) {
	r := newTestRegistry()

	err := r.Cancel(context.Background(), procedure.N2Handover)
	if !errors.Is(err, procedure.ErrNotActive) {
		t.Fatalf("expected ErrNotActive, got %v", err)
	}
}

func TestEndIsNoopWhenInactive(t *testing.T) {
	r := newTestRegistry()

	r.End(procedure.SecurityMode)

	if r.Active(procedure.SecurityMode) {
		t.Fatal("End on an idle slot marked it active")
	}

	if err := r.Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("End on an idle slot left it unusable: %v", err)
	}
}

func TestEndDoesNotInvokeCancel(t *testing.T) {
	r := newTestRegistry()

	var called atomic.Bool

	if err := r.Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.SecurityMode, time.Now().Add(time.Minute),
		func(context.Context) (procedure.Disposition, error) {
			called.Store(true)
			return procedure.Release, nil
		}); err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	r.End(procedure.SecurityMode)

	if called.Load() {
		t.Fatal("End should not invoke Cancel callback")
	}
}

func TestActiveTypes(t *testing.T) {
	r := newTestRegistry()

	if got := r.ActiveTypes(); got != nil {
		t.Fatalf("expected no active types, got %v", got)
	}

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	types := r.ActiveTypes()
	if len(types) != 1 || types[0] != string(procedure.N2Handover) {
		t.Fatalf("expected [N2Handover], got %v", types)
	}
}
