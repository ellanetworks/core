// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

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
	ctx := context.Background()

	id, err := r.Begin(ctx, procedure.Procedure{Type: procedure.Registration})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	if !r.Active(procedure.Registration) {
		t.Fatal("expected Registration to be active")
	}

	r.End(procedure.Registration)

	if r.Active(procedure.Registration) {
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

func TestConflictMatrix(t *testing.T) {
	tests := []struct {
		first   procedure.Type
		second  procedure.Type
		wantErr error
		desc    string
	}{
		{procedure.SecurityMode, procedure.N2Handover, procedure.ErrConflict, "C1: SMC blocks N2Handover"},
		{procedure.N2Handover, procedure.SecurityMode, procedure.ErrConflict, "C2: N2Handover blocks SMC"},
		{procedure.N2Handover, procedure.UEContextMod, procedure.ErrConflict, "C4: N2Handover blocks UEContextMod"},
		{procedure.UEContextMod, procedure.N2Handover, procedure.ErrConflict, "C4: UEContextMod blocks N2Handover"},
		{procedure.Registration, procedure.N2Handover, nil, "Registration allows N2Handover"},
		{procedure.Registration, procedure.Authentication, nil, "Registration allows Authentication"},
		{procedure.Registration, procedure.SecurityMode, nil, "Registration allows SecurityMode"},
		{procedure.Authentication, procedure.N2Handover, nil, "Authentication allows N2Handover"},
		{procedure.Paging, procedure.Registration, nil, "Paging allows Registration"},
		{procedure.Paging, procedure.N2Handover, nil, "Paging allows N2Handover"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			r := newTestRegistry()
			ctx := context.Background()

			_, err := r.Begin(ctx, procedure.Procedure{Type: tt.first})
			if err != nil {
				t.Fatalf("first Begin(%s) failed: %v", tt.first, err)
			}

			_, err = r.Begin(ctx, procedure.Procedure{Type: tt.second})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
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

func TestCancelAllReverseOrder(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	var (
		order []procedure.Type
		mu    sync.Mutex
	)

	makeCancel := func(typ procedure.Type) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()

			order = append(order, typ)

			return nil
		}
	}

	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Registration, Cancel: makeCancel(procedure.Registration)})
	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Authentication, Cancel: makeCancel(procedure.Authentication)})
	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Paging, Cancel: makeCancel(procedure.Paging)})

	r.CancelAll(ctx)

	expected := []procedure.Type{procedure.Paging, procedure.Authentication, procedure.Registration}
	if len(order) != len(expected) {
		t.Fatalf("expected %d cancellations, got %d", len(expected), len(order))
	}

	for i, typ := range expected {
		if order[i] != typ {
			t.Fatalf("position %d: expected %s, got %s", i, typ, order[i])
		}
	}
}

func TestPagingReentrant(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	id1, err := r.Begin(ctx, procedure.Procedure{Type: procedure.Paging})
	if err != nil {
		t.Fatalf("first Paging Begin failed: %v", err)
	}

	id2, err := r.Begin(ctx, procedure.Procedure{Type: procedure.Paging})
	if err != nil {
		t.Fatalf("second Paging Begin failed: %v", err)
	}

	if id1 == id2 {
		t.Fatal("expected different IDs for re-entrant instances")
	}

	r.EndByID(id1)

	if !r.Active(procedure.Paging) {
		t.Fatal("expected Paging still active (second instance)")
	}

	r.EndByID(id2)

	if r.Active(procedure.Paging) {
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
	r.End(procedure.Registration)
}

func TestEndDoesNotInvokeCancel(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	var called atomic.Bool

	_, err := r.Begin(ctx, procedure.Procedure{
		Type: procedure.Registration,
		Cancel: func(context.Context) error {
			called.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	r.End(procedure.Registration)

	if called.Load() {
		t.Fatal("End should not invoke Cancel callback")
	}
}

func TestSnapshot(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Registration})
	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Paging})

	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 entries in snapshot, got %d", len(snap))
	}

	for _, p := range snap {
		if p.Cancel != nil {
			t.Fatal("snapshot leaked Cancel callback")
		}
	}
}

func TestActiveTypes(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Registration})
	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Paging})
	_, _ = r.Begin(ctx, procedure.Procedure{Type: procedure.Paging})

	types := r.ActiveTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 unique types, got %d: %v", len(types), types)
	}
}

func TestCancelByID(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	var cancelledID atomic.Uint64

	id1, _ := r.Begin(ctx, procedure.Procedure{
		Type: procedure.Paging,
		Cancel: func(context.Context) error {
			cancelledID.Store(1)
			return nil
		},
	})

	id2, _ := r.Begin(ctx, procedure.Procedure{
		Type: procedure.Paging,
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

	if !r.Active(procedure.Paging) {
		t.Fatal("expected Paging still active (first instance)")
	}

	r.EndByID(id1)

	if r.Active(procedure.Paging) {
		t.Fatal("expected Paging inactive")
	}
}

func TestCancelFIFOForReentrant(t *testing.T) {
	r := newTestRegistry()
	ctx := context.Background()

	var (
		order []int
		mu    sync.Mutex
	)

	_, _ = r.Begin(ctx, procedure.Procedure{
		Type: procedure.Paging,
		Cancel: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()

			order = append(order, 1)

			return nil
		},
	})

	_, _ = r.Begin(ctx, procedure.Procedure{
		Type: procedure.Paging,
		Cancel: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()

			order = append(order, 2)

			return nil
		},
	})

	_ = r.Cancel(ctx, procedure.Paging)

	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("expected FIFO cancel of instance 1, got %v", order)
	}

	if !r.Active(procedure.Paging) {
		t.Fatal("expected second Paging instance still active")
	}
}
