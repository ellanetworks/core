// Copyright 2026 Ella Networks

package bgp

import (
	"context"
	"errors"
	"net/netip"
	"sort"
	"sync"
	"testing"
)

func TestReconcileDiff_EmptyBoth(t *testing.T) {
	ann, wd := reconcileDiff(map[string]string{}, map[string]string{})
	if len(ann) != 0 {
		t.Fatalf("expected no announces, got %d: %v", len(ann), ann)
	}

	if len(wd) != 0 {
		t.Fatalf("expected no withdraws, got %d: %v", len(wd), wd)
	}
}

func TestReconcileDiff_AllNew(t *testing.T) {
	desired := map[string]string{
		"10.0.0.1": "imsi-a",
		"10.0.0.2": "imsi-b",
	}
	ann, wd := reconcileDiff(desired, map[string]string{})

	if len(ann) != 2 {
		t.Fatalf("expected 2 announces, got %d: %v", len(ann), ann)
	}

	for ip, imsi := range desired {
		if got := ann[ip]; got != imsi {
			t.Errorf("announce for %s: expected imsi %q, got %q", ip, imsi, got)
		}
	}

	if len(wd) != 0 {
		t.Fatalf("expected no withdraws, got %d: %v", len(wd), wd)
	}
}

func TestReconcileDiff_AllGone(t *testing.T) {
	current := map[string]string{
		"10.0.0.1": "imsi-a",
		"10.0.0.2": "imsi-b",
	}
	ann, wd := reconcileDiff(map[string]string{}, current)

	if len(ann) != 0 {
		t.Fatalf("expected no announces, got %d: %v", len(ann), ann)
	}

	sort.Strings(wd)

	if len(wd) != 2 || wd[0] != "10.0.0.1" || wd[1] != "10.0.0.2" {
		t.Fatalf("expected withdraws for both IPs, got %v", wd)
	}
}

func TestReconcileDiff_Steady(t *testing.T) {
	m := map[string]string{
		"10.0.0.1": "imsi-a",
		"10.0.0.2": "imsi-b",
	}
	ann, wd := reconcileDiff(m, m)

	if len(ann) != 0 {
		t.Fatalf("expected no announces (steady state), got %v", ann)
	}

	if len(wd) != 0 {
		t.Fatalf("expected no withdraws (steady state), got %v", wd)
	}
}

func TestReconcileDiff_OwnershipFlip(t *testing.T) {
	// Same IP on both sides, but the IMSI changed. This mirrors the
	// failover case where a lease's owning node flips and the new
	// node announces the same /32 with an updated owner label.
	desired := map[string]string{"10.0.0.1": "imsi-new"}
	current := map[string]string{"10.0.0.1": "imsi-old"}

	ann, wd := reconcileDiff(desired, current)

	if got, want := ann["10.0.0.1"], "imsi-new"; got != want {
		t.Fatalf("expected announce with updated owner %q, got %q", want, got)
	}

	if len(wd) != 0 {
		t.Fatalf("expected no withdraw on ownership flip (re-announce is enough), got %v", wd)
	}
}

func TestReconcileDiff_Mixed(t *testing.T) {
	desired := map[string]string{
		"10.0.0.1": "imsi-a", // already announced, unchanged
		"10.0.0.2": "imsi-b", // new
	}
	current := map[string]string{
		"10.0.0.1": "imsi-a", // unchanged
		"10.0.0.3": "imsi-c", // no longer owned
	}

	ann, wd := reconcileDiff(desired, current)

	if len(ann) != 1 || ann["10.0.0.2"] != "imsi-b" {
		t.Fatalf("expected single announce of 10.0.0.2, got %v", ann)
	}

	if len(wd) != 1 || wd[0] != "10.0.0.3" {
		t.Fatalf("expected single withdraw of 10.0.0.3, got %v", wd)
	}
}

// fakeLeaseStore is a test LeaseStore that returns a fixed list.
type fakeLeaseStore struct {
	mu     sync.Mutex
	leases []Lease
	err    error
	calls  int
}

func (f *fakeLeaseStore) ListActiveLeasesByNode(_ context.Context, _ int) ([]Lease, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++

	if f.err != nil {
		return nil, f.err
	}

	out := make([]Lease, len(f.leases))
	copy(out, f.leases)

	return out, nil
}

func (f *fakeLeaseStore) set(leases []Lease) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.leases = leases
}

func TestReconciler_PopulatesEmptyRIB(t *testing.T) {
	svc := newTestBGPServiceAdvertising(t)

	store := &fakeLeaseStore{leases: []Lease{
		{Address: netip.MustParseAddr("10.45.0.1"), IMSI: "imsi-1"},
		{Address: netip.MustParseAddr("10.45.0.2"), IMSI: "imsi-2"},
	}}

	r := NewReconciler(svc, store, 1, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	paths := svc.Paths()
	if got, want := len(paths), 2; got != want {
		t.Fatalf("expected %d paths in RIB, got %d: %v", want, got, paths)
	}

	if paths["10.45.0.1"] != "imsi-1" {
		t.Errorf("10.45.0.1: expected imsi-1, got %q", paths["10.45.0.1"])
	}

	if paths["10.45.0.2"] != "imsi-2" {
		t.Errorf("10.45.0.2: expected imsi-2, got %q", paths["10.45.0.2"])
	}
}

func TestReconciler_ConvergesAfterChurn(t *testing.T) {
	svc := newTestBGPServiceAdvertising(t)

	store := &fakeLeaseStore{leases: []Lease{
		{Address: netip.MustParseAddr("10.45.0.1"), IMSI: "imsi-1"},
		{Address: netip.MustParseAddr("10.45.0.2"), IMSI: "imsi-2"},
	}}
	r := NewReconciler(svc, store, 1, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	// Drop one, add one.
	store.set([]Lease{
		{Address: netip.MustParseAddr("10.45.0.1"), IMSI: "imsi-1"},
		{Address: netip.MustParseAddr("10.45.0.3"), IMSI: "imsi-3"},
	})

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	paths := svc.Paths()
	if _, have := paths["10.45.0.1"]; !have {
		t.Errorf("10.45.0.1 should still be announced")
	}

	if _, have := paths["10.45.0.2"]; have {
		t.Errorf("10.45.0.2 should have been withdrawn: %v", paths)
	}

	if paths["10.45.0.3"] != "imsi-3" {
		t.Errorf("10.45.0.3: expected imsi-3, got %q", paths["10.45.0.3"])
	}
}

func TestReconciler_WithdrawsWhenLeaseTableEmpty(t *testing.T) {
	svc := newTestBGPServiceAdvertising(t)

	store := &fakeLeaseStore{leases: []Lease{
		{Address: netip.MustParseAddr("10.45.0.1"), IMSI: "imsi-1"},
	}}
	r := NewReconciler(svc, store, 1, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	if got := len(svc.Paths()); got != 1 {
		t.Fatalf("expected 1 path after initial reconcile, got %d", got)
	}

	store.set(nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if got := len(svc.Paths()); got != 0 {
		t.Fatalf("expected empty RIB after lease table cleared, got %d paths", got)
	}
}

func TestReconciler_StoreErrorPropagates(t *testing.T) {
	svc := newTestBGPServiceAdvertising(t)
	store := &fakeLeaseStore{err: errors.New("boom")}
	r := NewReconciler(svc, store, 1, nil)

	err := r.Reconcile(context.Background())
	if err == nil {
		t.Fatal("expected error to propagate from lease store")
	}
}

func TestReconciler_StartStopIsIdempotent(t *testing.T) {
	svc := newTestBGPServiceAdvertising(t)
	store := &fakeLeaseStore{}
	r := NewReconciler(svc, store, 1, nil)

	r.Start()
	r.Start() // second call is a no-op
	r.Stop()
	r.Stop() // no-op when not running
}

// fakeRIB is an in-memory implementation of RIB for tests. Thread-safe.
type fakeRIB struct {
	mu    sync.Mutex
	paths map[string]string
}

func newFakeRIB() *fakeRIB {
	return &fakeRIB{paths: make(map[string]string)}
}

func (f *fakeRIB) Announce(ip netip.Addr, owner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.paths[ip.String()] = owner

	return nil
}

func (f *fakeRIB) Withdraw(ip netip.Addr) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.paths, ip.String())

	return nil
}

func (f *fakeRIB) Paths() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make(map[string]string, len(f.paths))
	for k, v := range f.paths {
		out[k] = v
	}

	return out
}

func newTestBGPServiceAdvertising(t *testing.T) RIB {
	t.Helper()
	return newFakeRIB()
}
