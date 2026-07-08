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
	engine "github.com/ellanetworks/core/internal/procedure"
	"go.uber.org/zap"
)

func newTestRegistry() *procedure.Registry {
	return procedure.NewRegistry(zap.NewNop())
}

// reentrantTestType and newReentrantTestRegistry exercise the shared engine's
// re-entrancy / multi-instance mechanism (EndByID, CancelByID, FIFO cancel)
// without any production type declaring itself reentrant — the AMF's own type set
// is all mutually exclusive.
const reentrantTestType procedure.Type = "ReentrantTest"

type reentrantTestRules struct{}

func (reentrantTestRules) Conflicts(procedure.Type, procedure.Type) (bool, string) {
	return false, ""
}

func (reentrantTestRules) Reentrant(t procedure.Type) bool { return t == reentrantTestType }

func newReentrantTestRegistry() *procedure.Registry {
	return engine.NewRegistry(zap.NewNop(), reentrantTestRules{})
}

func TestBeginEndRoundTrip(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	id, err := r.Begin(ctx, procedure.Procedure{Type: procedure.SecurityMode})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	if !r.Active(procedure.SecurityMode) {
		t.Fatal("expected Registration to be active")
	}

	r.End(procedure.SecurityMode)

	if r.Active(procedure.SecurityMode) {
		t.Fatal("expected Registration to be inactive after End")
	}
}

func TestSameTypeConflict(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	_, err := r.Begin(ctx, procedure.Procedure{Type: procedure.N2Handover})
	if err != nil {
		t.Fatalf("first Begin failed: %v", err)
	}

	_, err = r.Begin(ctx, procedure.Procedure{Type: procedure.N2Handover})
	if !errors.Is(err, procedure.ErrAlreadyActive) {
		t.Fatalf("expected ErrAlreadyActive, got %v", err)
	}
}

// TestKeyChainMutualExclusion checks the coarse rule (TS 33.501 §6.9.5): every ordered
// pair of the tracked key-changing procedures {SecurityMode, N2Handover, PathSwitch} is
// mutually exclusive, since they all mutate the one {NH,NCC}/KgNB chain.
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
			ctx := context.Background()

			if _, err := r.Begin(ctx, procedure.Procedure{Type: tt.first}); err != nil {
				t.Fatalf("first Begin(%s) failed: %v", tt.first, err)
			}

			_, err := r.Begin(ctx, procedure.Procedure{Type: tt.second})
			if !errors.Is(err, procedure.ErrConflict) {
				t.Fatalf("%s: expected ErrConflict, got %v", tt.desc, err)
			}
		})
	}
}

func TestContextCancellationAborts(t *testing.T) {
	r := newTestRegistry()
	ctx, cancel := context.WithCancel(context.Background())

	var cancelled atomic.Bool

	_, err := r.Begin(ctx, procedure.Procedure{
		Type: procedure.N2Handover,
		Cancel: func(context.Context) error {
			cancelled.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	cancel()

	deadline := time.Now().Add(time.Second)
	for !cancelled.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if !cancelled.Load() {
		t.Fatal("expected cancel callback to fire on ctx cancellation")
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover to be removed after ctx cancellation")
	}
}

func TestContextNotCancelledDoesNotFire(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	var cancelled atomic.Bool

	_, err := r.Begin(ctx, procedure.Procedure{
		Type: procedure.N2Handover,
		Cancel: func(context.Context) error {
			cancelled.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	r.End(procedure.N2Handover)

	time.Sleep(20 * time.Millisecond)

	if cancelled.Load() {
		t.Fatal("Cancel callback fired after End")
	}
}

func TestDeadlineExpiry(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	var cancelled atomic.Bool

	_, err := r.Begin(ctx, procedure.Procedure{
		Type:     procedure.N2Handover,
		Deadline: time.Now().Add(50 * time.Millisecond),
		Cancel: func(context.Context) error {
			cancelled.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if !cancelled.Load() {
		t.Fatal("expected cancel callback to be invoked on timeout")
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover to be removed after timeout")
	}
}

func TestSuperviseArmsDeadlineAfterBegin(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	if _, err := r.Begin(ctx, procedure.Procedure{Type: procedure.N2Handover}); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	var cancelled atomic.Bool

	err := r.Supervise(ctx, procedure.N2Handover, time.Now().Add(50*time.Millisecond),
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
	ctx := context.Background()

	if _, err := r.Begin(ctx, procedure.Procedure{Type: procedure.N2Handover}); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	var cancelled atomic.Bool

	if err := r.Supervise(ctx, procedure.N2Handover, time.Now().Add(50*time.Millisecond),
		func(context.Context) error { cancelled.Store(true); return nil }); err != nil {
		t.Fatalf("Supervise failed: %v", err)
	}

	r.End(procedure.N2Handover)

	time.Sleep(150 * time.Millisecond)

	if cancelled.Load() {
		t.Fatal("End must stop the supervision timer so the cancel never fires")
	}
}

func TestSuperviseNotActive(t *testing.T) {
	r := newTestRegistry()

	err := r.Supervise(context.Background(), procedure.N2Handover, time.Now().Add(time.Minute),
		func(context.Context) error { return nil })
	if !errors.Is(err, procedure.ErrNotActive) {
		t.Fatalf("expected ErrNotActive, got %v", err)
	}
}

func TestCancelCallbackPanicRecovery(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	_, err := r.Begin(ctx, procedure.Procedure{
		Type: procedure.N2Handover,
		Cancel: func(context.Context) error {
			panic("intentional panic")
		},
	})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	err = r.Cancel(ctx, procedure.N2Handover)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover removed even after panic")
	}
}

func TestCancelCallbackErrorStillRemoves(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	_, err := r.Begin(ctx, procedure.Procedure{
		Type: procedure.N2Handover,
		Cancel: func(context.Context) error {
			return errors.New("cancel failed")
		},
	})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	err = r.Cancel(ctx, procedure.N2Handover)
	if err != nil {
		t.Fatalf("Cancel should succeed even if callback errors: %v", err)
	}

	if r.Active(procedure.N2Handover) {
		t.Fatal("expected N2Handover removed even after callback error")
	}
}

func TestConcurrentBeginConflict(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	const n = 50

	var (
		successes atomic.Int32
		wg        sync.WaitGroup
	)

	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()

			_, err := r.Begin(ctx, procedure.Procedure{Type: procedure.N2Handover})
			if err == nil {
				successes.Add(1)
			}
		}()
	}

	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("expected exactly 1 success, got %d", got)
	}
}

func TestPagingReentrant(t *testing.T) {
	r := newReentrantTestRegistry()
	ctx := context.Background()

	id1, err := r.Begin(ctx, procedure.Procedure{Type: reentrantTestType})
	if err != nil {
		t.Fatalf("first Paging Begin failed: %v", err)
	}

	id2, err := r.Begin(ctx, procedure.Procedure{Type: reentrantTestType})
	if err != nil {
		t.Fatalf("second Paging Begin failed: %v", err)
	}

	if id1 == id2 {
		t.Fatal("expected different IDs for re-entrant instances")
	}

	r.EndByID(id1)

	if !r.Active(reentrantTestType) {
		t.Fatal("expected Paging still active (second instance)")
	}

	r.EndByID(id2)

	if r.Active(reentrantTestType) {
		t.Fatal("expected Paging inactive after both ended")
	}
}

func TestCancelNotActive(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	err := r.Cancel(ctx, procedure.N2Handover)
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
	ctx := context.Background()

	var called atomic.Bool

	_, err := r.Begin(ctx, procedure.Procedure{
		Type: procedure.SecurityMode,
		Cancel: func(context.Context) error {
			called.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	r.End(procedure.SecurityMode)

	if called.Load() {
		t.Fatal("End should not invoke Cancel callback")
	}
}

func TestActiveTypes(t *testing.T) {
	r := newReentrantTestRegistry()
	ctx := context.Background()

	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.SecurityMode})
	_, _ = r.Begin(ctx, procedure.Procedure{Type: reentrantTestType})
	_, _ = r.Begin(ctx, procedure.Procedure{Type: reentrantTestType})

	types := r.ActiveTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 unique types, got %d: %v", len(types), types)
	}
}

func TestCancelByID(t *testing.T) {
	r := newReentrantTestRegistry()
	ctx := context.Background()

	var cancelledID atomic.Uint64

	id1, _ := r.Begin(ctx, procedure.Procedure{
		Type: reentrantTestType,
		Cancel: func(context.Context) error {
			cancelledID.Store(1)
			return nil
		},
	})

	id2, _ := r.Begin(ctx, procedure.Procedure{
		Type: reentrantTestType,
		Cancel: func(context.Context) error {
			cancelledID.Store(2)
			return nil
		},
	})

	err := r.CancelByID(ctx, id2)
	if err != nil {
		t.Fatalf("CancelByID failed: %v", err)
	}

	if cancelledID.Load() != 2 {
		t.Fatalf("expected cancel of instance 2, got %d", cancelledID.Load())
	}

	if !r.Active(reentrantTestType) {
		t.Fatal("expected Paging still active (first instance)")
	}

	r.EndByID(id1)

	if r.Active(reentrantTestType) {
		t.Fatal("expected Paging inactive")
	}
}

func TestCancelFIFOForReentrant(t *testing.T) {
	r := newReentrantTestRegistry()
	ctx := context.Background()

	var (
		order []int
		mu    sync.Mutex
	)

	_, _ = r.Begin(ctx, procedure.Procedure{
		Type: reentrantTestType,
		Cancel: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()

			order = append(order, 1)

			return nil
		},
	})

	_, _ = r.Begin(ctx, procedure.Procedure{
		Type: reentrantTestType,
		Cancel: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()

			order = append(order, 2)

			return nil
		},
	})

	_ = r.Cancel(ctx, reentrantTestType)

	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("expected FIFO cancel of instance 1, got %v", order)
	}

	if !r.Active(reentrantTestType) {
		t.Fatal("expected second Paging instance still active")
	}
}
