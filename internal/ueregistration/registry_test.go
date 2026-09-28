// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration_test

import (
	"context"
	"errors"
	"sort"
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
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]db.UERegistration{}, version: 100}
}

func rowKey(imsi, regType string) string { return imsi + "/" + regType }

func (f *fakeStore) RegisterUE(_ context.Context, imsi, regType, nodeID string, cancel ...string) (int64, error) {
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

	for _, c := range cancel {
		if other, ok := f.rows[rowKey(imsi, c)]; ok && !other.Purged {
			other.Purged = true
			other.Version = f.version
			f.rows[rowKey(imsi, c)] = other
		}
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

func (f *fakeStore) ListUERegistrationsSince(_ context.Context, version int64) ([]db.UERegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []db.UERegistration

	for _, row := range f.rows {
		if row.Version > version {
			out = append(out, row)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })

	return out, nil
}

func (f *fakeStore) MaxUERegistrationVersion(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var highest int64

	for _, row := range f.rows {
		if row.Version > highest {
			highest = row.Version
		}
	}

	return highest, nil
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
	mu         sync.Mutex
	holds      map[string]bool
	reconciled chan string
	all        chan struct{}
}

func newFakeHolder() *fakeHolder {
	return &fakeHolder{
		holds:      map[string]bool{},
		reconciled: make(chan string, 16),
		all:        make(chan struct{}, 4),
	}
}

func (h *fakeHolder) HoldsUE(imsi string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.holds[imsi]
}

func (h *fakeHolder) ReconcileRegistration(_ context.Context, imsi string) {
	h.reconciled <- imsi
}

func (h *fakeHolder) ReconcileRegistrations(context.Context) {
	h.all <- struct{}{}
}

func newAMFBinding(store ueregistration.Store) (*ueregistration.Registry, *ueregistration.Binding, *fakeHolder) {
	r := ueregistration.New(store, "node-a", zap.NewNop())
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
		name string
		amf  *row
		mme  *row
		held int64
		want bool
	}{
		{"registered here", &row{"node-a", false, 10}, nil, 10, false},
		{"never confirmed", &row{"node-b", false, 20}, nil, 0, false},
		{"newer registration elsewhere", &row{"node-b", false, 20}, nil, 10, true},
		{"older registration elsewhere", &row{"node-b", false, 5}, nil, 10, false},
		{"purged registration elsewhere", &row{"node-b", true, 20}, nil, 10, false},
		{"no registration", nil, nil, 10, false},
		{"cancelled by EPS elsewhere", &row{"node-a", true, 20}, &row{"node-b", false, 20}, 10, true},
		{"cancelled by EPS here", &row{"node-a", true, 20}, &row{"node-a", false, 20}, 10, false},
		{"cancelled, EPS since purged", &row{"node-a", true, 20}, &row{"node-b", true, 30}, 10, true},
		{"cancelled, no EPS registration", &row{"node-a", true, 20}, nil, 10, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			if tc.amf != nil {
				store.set(amfType, tc.amf.nodeID, tc.amf.purged, tc.amf.version)
			}

			if tc.mme != nil {
				store.set(mmeType, tc.mme.nodeID, tc.mme.purged, tc.mme.version)
			}

			_, b, _ := newAMFBinding(store)

			released := false

			b.Reconcile(context.Background(), imsi, func() int64 { return tc.held }, func(context.Context) { released = true })

			if released != tc.want {
				t.Fatalf("released = %v, want %v", released, tc.want)
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
		})
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

func TestRun_ReconcilesChangedSubscribersOnly(t *testing.T) {
	store := newFakeStore()
	store.set(amfType, "node-b", false, 50)

	r, _, h := newAMFBinding(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wakeup := make(chan struct{}, 1)

	go r.Run(ctx, wakeup)

	time.Sleep(20 * time.Millisecond)

	const changed = "001010000000002"

	store.mu.Lock()
	store.rows[rowKey(changed, mmeType)] = db.UERegistration{Imsi: changed, Type: mmeType, NodeID: "node-b", Version: 60}
	store.mu.Unlock()

	wakeup <- struct{}{}

	select {
	case got := <-h.reconciled:
		if got != changed {
			t.Fatalf("reconciled %s, want only the changed subscriber %s", got, changed)
		}
	case <-time.After(time.Second):
		t.Fatal("expected the changed subscriber to be reconciled")
	}

	select {
	case got := <-h.reconciled:
		t.Fatalf("unexpected reconcile of %s", got)
	case <-time.After(50 * time.Millisecond):
	}
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
