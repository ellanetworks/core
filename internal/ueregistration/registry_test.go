// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/ueregistration"
	"go.uber.org/zap"
)

const (
	imsi    = "001010000000001"
	amfType = db.UERegistrationTypeAMF3GPPAccess
	mmeType = db.UERegistrationTypeMME
)

type fakeStore struct {
	mu       sync.Mutex
	rows     map[string]db.UERegistration
	version  int64
	err      error
	purgeErr error
	purges   int
	subErr   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]db.UERegistration{}, version: 100}
}

func rowKey(imsi, regType string) string { return imsi + "/" + regType }

func (f *fakeStore) RegisterUE(_ context.Context, imsi, regType, nodeID, cancel string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return 0, f.err
	}

	f.version++

	row, ok := f.rows[rowKey(imsi, regType)]
	if !ok || row.NodeID != nodeID || row.Purged {
		row = db.UERegistration{Imsi: imsi, Type: regType, NodeID: nodeID, Version: f.version}
		f.rows[rowKey(imsi, regType)] = row
	}

	if other, ok := f.rows[rowKey(imsi, cancel)]; ok && !other.Purged {
		other.Purged = true
		other.Version = f.version
		f.rows[rowKey(imsi, cancel)] = other
	}

	return row.Version, nil
}

func (f *fakeStore) PurgeUERegistration(_ context.Context, imsi, regType, nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.purges++

	if f.purgeErr != nil {
		return f.purgeErr
	}

	if row, ok := f.rows[rowKey(imsi, regType)]; ok && row.NodeID == nodeID && !row.Purged {
		f.version++
		row.Purged = true
		row.Version = f.version
		f.rows[rowKey(imsi, regType)] = row
	}

	return nil
}

func (f *fakeStore) GetUERegistration(_ context.Context, imsi, regType string) (*db.UERegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.rows[rowKey(imsi, regType)]
	if !ok {
		return nil, db.ErrNotFound
	}

	return &row, nil
}

func (f *fakeStore) GetSubscriber(_ context.Context, imsi string) (*db.Subscriber, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.subErr != nil {
		return nil, f.subErr
	}

	return &db.Subscriber{Imsi: imsi}, nil
}

func (f *fakeStore) set(regType, nodeID string, purged bool, version int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.rows[rowKey(imsi, regType)] = db.UERegistration{Imsi: imsi, Type: regType, NodeID: nodeID, Purged: purged, Version: version}
}

func (f *fakeStore) get(regType string) (db.UERegistration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.rows[rowKey(imsi, regType)]

	return row, ok
}

type fakeHolder struct {
	mu    sync.Mutex
	holds map[string]bool
	all   chan struct{}
}

func newFakeHolder() *fakeHolder {
	return &fakeHolder{
		holds: map[string]bool{},
		all:   make(chan struct{}, 4),
	}
}

func (h *fakeHolder) HoldsUE(imsi string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.holds[imsi]
}

func (h *fakeHolder) ReconcileRegistrations(context.Context) {
	select {
	case h.all <- struct{}{}:
	default:
	}
}

func newAMFBinding(store ueregistration.Store) (*ueregistration.Registry, *ueregistration.Binding, *fakeHolder) {
	r := ueregistration.New(store, "node-a", zap.NewNop())
	r.SetIntervalForTest(20 * time.Millisecond)

	h := newFakeHolder()

	return r, r.Bind(amfType, mmeType, h), h
}

func TestRegister_CancelsOtherType(t *testing.T) {
	store := newFakeStore()
	store.set(mmeType, "node-b", false, 10)

	_, b, _ := newAMFBinding(store)

	version, err := b.Register(context.Background(), imsi)
	if err != nil {
		t.Fatalf("Register: %s", err)
	}

	amf, _ := store.get(amfType)
	if amf.NodeID != "node-a" || amf.Purged || amf.Version != version {
		t.Fatalf("unexpected AMF registration %+v (returned version %d)", amf, version)
	}

	if mme, _ := store.get(mmeType); !mme.Purged {
		t.Fatal("expected the MME registration to be cancelled")
	}
}

func TestRegister_MigrationPending(t *testing.T) {
	store := newFakeStore()
	store.err = db.ErrMigrationPending

	_, b, _ := newAMFBinding(store)

	version, err := b.Register(context.Background(), imsi)
	if err != nil || version != 0 {
		t.Fatalf("Register = (%d, %v), want (0, nil)", version, err)
	}
}

func TestRegister_StoreError(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("no leader")

	_, b, _ := newAMFBinding(store)

	if _, err := b.Register(context.Background(), imsi); err == nil {
		t.Fatal("expected error")
	}
}

func TestConfirmed(t *testing.T) {
	type row struct {
		nodeID string
		purged bool
	}

	cases := []struct {
		name    string
		row     *row
		version int64
		want    bool
	}{
		{"registered here", &row{"node-a", false}, 5, true},
		{"never confirmed", &row{"node-a", false}, 0, false},
		{"purged", &row{"node-a", true}, 5, false},
		{"registered elsewhere", &row{"node-b", false}, 5, false},
		{"no registration", nil, 5, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			if tc.row != nil {
				store.set(amfType, tc.row.nodeID, tc.row.purged, 5)
			}

			_, b, _ := newAMFBinding(store)

			if got := b.Confirmed(context.Background(), imsi, tc.version); got != tc.want {
				t.Fatalf("Confirmed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	type row struct {
		nodeID  string
		purged  bool
		version int64
	}

	cases := []struct {
		name   string
		amf    *row
		mme    *row
		subErr error
		held   int64
		want   string
	}{
		{"registered here", &row{"node-a", false, 10}, nil, nil, 10, "kept"},
		{"never confirmed", &row{"node-b", false, 20}, nil, nil, 0, "kept"},
		{"newer registration elsewhere", &row{"node-b", false, 20}, nil, nil, 10, "released"},
		{"older registration elsewhere", &row{"node-b", false, 5}, nil, nil, 10, "kept"},
		{"purged registration elsewhere", &row{"node-b", true, 20}, nil, nil, 10, "kept"},
		{"no registration", nil, nil, nil, 10, "kept"},
		{"cancelled by EPS elsewhere", &row{"node-a", true, 20}, &row{"node-b", false, 20}, nil, 10, "released"},
		{"cancelled by EPS here", &row{"node-a", true, 20}, &row{"node-a", false, 20}, nil, 10, "kept"},
		{"cancelled, EPS since purged", &row{"node-a", true, 20}, &row{"node-b", true, 30}, nil, 10, "released"},
		{"cancelled, no EPS registration", &row{"node-a", true, 20}, nil, nil, 10, "released"},
		{"subscriber deleted", nil, nil, db.ErrNotFound, 10, "withdrawn"},
		{"subscriber deleted, never confirmed", nil, nil, db.ErrNotFound, 0, "kept"},
		{"subscriber lookup failed", nil, nil, errors.New("db unavailable"), 10, "kept"},
		{"subscriber deleted, registration still here", &row{"node-a", false, 10}, nil, db.ErrNotFound, 10, "kept"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			store.subErr = tc.subErr

			if tc.amf != nil {
				store.set(amfType, tc.amf.nodeID, tc.amf.purged, tc.amf.version)
			}

			if tc.mme != nil {
				store.set(mmeType, tc.mme.nodeID, tc.mme.purged, tc.mme.version)
			}

			_, b, _ := newAMFBinding(store)

			got := "kept"

			b.Reconcile(context.Background(), imsi, func() int64 { return tc.held },
				func(context.Context) { got = "released" },
				func(context.Context) { got = "withdrawn" })

			if got != tc.want {
				t.Fatalf("outcome = %s, want %s", got, tc.want)
			}

			if _, ok := store.get(amfType); tc.amf == nil && ok {
				t.Fatal("a missing registration must not be recreated by the reconcile")
			}
		})
	}
}

func TestReconcile_SerializesWithRegister(t *testing.T) {
	store := newFakeStore()
	store.set(amfType, "node-b", false, 20)

	_, b, _ := newAMFBinding(store)

	inRelease := make(chan struct{})
	unblock := make(chan struct{})
	reconciled := make(chan struct{})

	go func() {
		b.Reconcile(context.Background(), imsi, func() int64 { return 10 }, func(context.Context) {
			close(inRelease)
			<-unblock
		}, func(context.Context) {})
		close(reconciled)
	}()

	<-inRelease

	registered := make(chan struct{})

	go func() {
		if _, err := b.Register(context.Background(), imsi); err != nil {
			t.Errorf("Register: %s", err)
		}

		close(registered)
	}()

	select {
	case <-registered:
		t.Fatal("Register ran while the release held the subscriber's lock")
	case <-time.After(50 * time.Millisecond):
	}

	close(unblock)
	<-reconciled
	<-registered
}

func TestRun_PurgesWhenNoContextIsHeld(t *testing.T) {
	store := newFakeStore()
	store.set(amfType, "node-a", false, 10)

	r, b, h := newAMFBinding(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx, nil)

	h.mu.Lock()
	h.holds[imsi] = true
	h.mu.Unlock()

	b.Purge(imsi)

	time.Sleep(50 * time.Millisecond)

	store.mu.Lock()
	purges := store.purges
	store.mu.Unlock()

	if row, _ := store.get(amfType); row.Purged || purges != 0 {
		t.Fatal("a held UE must not be purged")
	}

	h.mu.Lock()
	h.holds[imsi] = false
	h.mu.Unlock()

	b.Purge(imsi)

	waitFor(t, func() bool {
		row, _ := store.get(amfType)
		return row.Purged
	})
}

func TestRun_RetriesFailedPurges(t *testing.T) {
	store := newFakeStore()
	store.set(amfType, "node-a", false, 10)
	store.purgeErr = errors.New("no leader")

	r, b, _ := newAMFBinding(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx, nil)

	b.Purge(imsi)

	waitFor(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()

		return store.purges >= 1
	})

	store.mu.Lock()
	store.purgeErr = nil
	store.mu.Unlock()

	b.Purge("001010000000002")

	waitFor(t, func() bool {
		row, _ := store.get(amfType)
		return row.Purged
	})
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("condition not met")
}

func TestConfirmed_RestoredRegistrationIsNotConfirmed(t *testing.T) {
	store := newFakeStore()
	store.set(amfType, "node-a", false, 0)

	_, b, _ := newAMFBinding(store)

	if b.Confirmed(context.Background(), imsi, 5) {
		t.Fatal("a registration restored from a backup must be confirmed again")
	}
}

func TestRun_StopsFlushAtFirstFailure(t *testing.T) {
	store := newFakeStore()
	store.purgeErr = errors.New("no leader")

	r, b, _ := newAMFBinding(store)
	r.SetIntervalForTest(200 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.Purge(imsi)
	b.Purge("001010000000002")

	go r.Run(ctx, nil)

	waitFor(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()

		return store.purges >= 1
	})

	time.Sleep(50 * time.Millisecond)

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.purges != 1 {
		t.Fatalf("purge attempts = %d, want 1: a failing leader must not be retried for every pending purge", store.purges)
	}
}

func TestRun_WakeupReconcilesHeldUEs(t *testing.T) {
	store := newFakeStore()

	r := ueregistration.New(store, "node-a", zap.NewNop())
	h := newFakeHolder()
	r.Bind(amfType, mmeType, h)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wakeup := make(chan struct{}, 1)

	go r.Run(ctx, wakeup)

	wakeup <- struct{}{}

	select {
	case <-h.all:
	case <-time.After(time.Second):
		t.Fatal("expected a wakeup to reconcile the held UEs")
	}
}

func TestRun_TickReconcilesHeldUEs(t *testing.T) {
	store := newFakeStore()

	r, _, h := newAMFBinding(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx, nil)

	select {
	case <-h.all:
	case <-time.After(time.Second):
		t.Fatal("expected the periodic tick to reconcile the held UEs, retrying any skipped release")
	}
}
