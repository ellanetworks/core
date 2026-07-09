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

// TestKeyChainMutualExclusion checks the coarse rule (TS 33.501 §6.9.5): every ordered
// pair of the tracked key-changing procedures {SecurityMode, N2Handover, PathSwitch} is
// mutually exclusive, since they all mutate the one {NH,NCC}/KgNB chain — at most one is
// active at a time.
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
		func(context.Context) error { cancelled.Store(true); return nil })
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
		func(context.Context) error { cancelled.Store(true); return nil }); err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	r.End(procedure.N2Handover)

	time.Sleep(150 * time.Millisecond)

	if cancelled.Load() {
		t.Fatal("End must stop the supervision timer so the cancel never fires")
	}
}

// TestStaleSuperviseTimerDoesNotExpireRebegun verifies the pointer-identity guard:
// after End stops one instance, a new procedure of the same Type must not be torn down
// by the previous instance's supervision.
func TestStaleSuperviseTimerDoesNotExpireRebegun(t *testing.T) {
	r := newTestRegistry()

	var fired atomic.Bool

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("first Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(40*time.Millisecond),
		func(context.Context) error { fired.Store(true); return nil }); err != nil {
		t.Fatalf("first Supervise failed: %v", err)
	}

	r.End(procedure.N2Handover)

	if err := r.Begin(procedure.N2Handover); err != nil {
		t.Fatalf("second Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.N2Handover, time.Now().Add(400*time.Millisecond),
		func(context.Context) error { fired.Store(true); return nil }); err != nil {
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
		func(context.Context) error { return nil })
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
		func(context.Context) error { called.Store(true); return nil }); err != nil {
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
		func(context.Context) error { panic("intentional panic") }); err != nil {
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
		func(context.Context) error { return errors.New("cancel failed") }); err != nil {
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
}

func TestEndDoesNotInvokeCancel(t *testing.T) {
	r := newTestRegistry()

	var called atomic.Bool

	if err := r.Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := r.Supervise(procedure.SecurityMode, time.Now().Add(time.Minute),
		func(context.Context) error { called.Store(true); return nil }); err != nil {
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
