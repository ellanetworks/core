// Copyright 2026 Ella Networks

package ipam

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// fakeStore — in-memory LeaseStore for unit testing
// ---------------------------------------------------------------------------

type fakeStore struct {
	mu     sync.Mutex
	leases []Lease
	nextID int
}

func newFakeStore() *fakeStore {
	return &fakeStore{nextID: 1}
}

func (s *fakeStore) GetDynamicLease(_ context.Context, poolID int, imsi string) (*Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.leases {
		if s.leases[i].PoolID == poolID && s.leases[i].IMSI == imsi && s.leases[i].Type == "dynamic" {
			return &s.leases[i], nil
		}
	}

	return nil, ErrNotFound
}

func (s *fakeStore) GetLeaseBySession(_ context.Context, poolID int, sessionID int, imsi string) (*Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.leases {
		l := &s.leases[i]
		if l.PoolID == poolID && l.IMSI == imsi && l.SessionID != nil && *l.SessionID == sessionID {
			return l, nil
		}
	}

	return nil, ErrNotFound
}

func (s *fakeStore) ListLeaseAddressesByPool(_ context.Context, poolID int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var addrs []string

	for _, l := range s.leases {
		if l.PoolID == poolID {
			addrs = append(addrs, l.Address)
		}
	}

	sort.Strings(addrs)

	return addrs, nil
}

func (s *fakeStore) CreateLease(_ context.Context, lease *Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, l := range s.leases {
		if l.PoolID == lease.PoolID && l.Address == lease.Address {
			return ErrAlreadyExists
		}
	}

	lease.ID = s.nextID
	s.nextID++
	s.leases = append(s.leases, *lease)

	return nil
}

func (s *fakeStore) UpdateLeaseSession(_ context.Context, leaseID int, sessionID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.leases {
		if s.leases[i].ID == leaseID {
			s.leases[i].SessionID = &sessionID
			return nil
		}
	}

	return ErrNotFound
}

func (s *fakeStore) DeleteDynamicLease(_ context.Context, leaseID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.leases {
		if s.leases[i].ID == leaseID && s.leases[i].Type == "dynamic" {
			s.leases = append(s.leases[:i], s.leases[i+1:]...)
			return nil
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func mustPool(cidr string) Pool {
	p, err := NewPool(1, cidr)
	if err != nil {
		panic(err)
	}

	return p
}

func TestAllocate_FirstAddress(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/24")

	addr, err := alloc.Allocate(context.Background(), pool, "001010000000001", 1)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// First usable address is 192.168.1.1.
	if addr.String() != "192.168.1.1" {
		t.Fatalf("expected 192.168.1.1, got %s", addr)
	}
}

func TestAllocate_Sequential(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/24")

	addrs := make(map[string]bool)

	for i := 1; i <= 5; i++ {
		imsi := fmt.Sprintf("0010100000000%02d", i)

		addr, err := alloc.Allocate(context.Background(), pool, imsi, i)
		if err != nil {
			t.Fatalf("Allocate #%d: %v", i, err)
		}

		if addrs[addr.String()] {
			t.Fatalf("duplicate address: %s", addr)
		}

		addrs[addr.String()] = true
	}

	// Should have allocated 192.168.1.1 through 192.168.1.5 sequentially.
	for i := 1; i <= 5; i++ {
		expected := fmt.Sprintf("192.168.1.%d", i)
		if !addrs[expected] {
			t.Fatalf("expected %s to be allocated", expected)
		}
	}
}

func TestAllocate_PoolExhaustion(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/29") // 6 usable addresses

	for i := 1; i <= 6; i++ {
		imsi := fmt.Sprintf("0010100000000%02d", i)

		_, err := alloc.Allocate(context.Background(), pool, imsi, i)
		if err != nil {
			t.Fatalf("Allocate #%d: %v", i, err)
		}
	}

	// 7th allocation should fail.
	_, err := alloc.Allocate(context.Background(), pool, "001010000000007", 7)
	if err != ErrPoolExhausted {
		t.Fatalf("expected ErrPoolExhausted, got %v", err)
	}
}

func TestAllocate_SkipsNetworkAndBroadcast(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("10.0.0.0/30") // 4 total: .0 (network), .1, .2, .3 (broadcast) — 2 usable

	addr1, err := alloc.Allocate(context.Background(), pool, "001010000000001", 1)
	if err != nil {
		t.Fatalf("Allocate 1: %v", err)
	}

	if addr1.String() != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %s", addr1)
	}

	addr2, err := alloc.Allocate(context.Background(), pool, "001010000000002", 2)
	if err != nil {
		t.Fatalf("Allocate 2: %v", err)
	}

	if addr2.String() != "10.0.0.2" {
		t.Fatalf("expected 10.0.0.2, got %s", addr2)
	}

	// Pool should now be exhausted.
	_, err = alloc.Allocate(context.Background(), pool, "001010000000003", 3)
	if err != ErrPoolExhausted {
		t.Fatalf("expected ErrPoolExhausted, got %v", err)
	}
}

func TestAllocate_LeaseReuse_ReRegistration(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/24")

	// Initial allocation.
	addr1, err := alloc.Allocate(context.Background(), pool, "001010000000001", 1)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// Re-registration with a new session for the same IMSI.
	addr2, err := alloc.Allocate(context.Background(), pool, "001010000000001", 2)
	if err != nil {
		t.Fatalf("Allocate (re-registration): %v", err)
	}

	// Should reuse the same address.
	if addr1 != addr2 {
		t.Fatalf("expected lease re-use: %s vs %s", addr1, addr2)
	}
}

func TestAllocate_GapFilling(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/24")

	// Allocate 3 addresses.
	for i := 1; i <= 3; i++ {
		imsi := fmt.Sprintf("0010100000000%02d", i)

		_, err := alloc.Allocate(context.Background(), pool, imsi, i)
		if err != nil {
			t.Fatalf("Allocate #%d: %v", i, err)
		}
	}

	// Release the middle one (192.168.1.2).
	released, err := alloc.Release(context.Background(), pool, 2, "001010000000002")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}

	if released.String() != "192.168.1.2" {
		t.Fatalf("expected released 192.168.1.2, got %s", released)
	}

	// Next allocation should fill the gap.
	addr, err := alloc.Allocate(context.Background(), pool, "001010000000004", 4)
	if err != nil {
		t.Fatalf("Allocate (gap): %v", err)
	}

	if addr.String() != "192.168.1.2" {
		t.Fatalf("expected gap fill at 192.168.1.2, got %s", addr)
	}
}

func TestRelease_Dynamic(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/24")

	addr, _ := alloc.Allocate(context.Background(), pool, "001010000000001", 1)

	released, err := alloc.Release(context.Background(), pool, 1, "001010000000001")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}

	if released != addr {
		t.Fatalf("expected %s, got %s", addr, released)
	}

	// Lease should be gone.
	_, err = store.GetDynamicLease(context.Background(), pool.ID, "001010000000001")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRelease_NotFound(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/24")

	_, err := alloc.Release(context.Background(), pool, 999, "001010000000001")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestAllocate_ConcurrentRaces(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("192.168.1.0/24")

	const numGoroutines = 20

	var wg sync.WaitGroup

	results := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			imsi := fmt.Sprintf("001010000000%03d", idx+1)

			addr, err := alloc.Allocate(context.Background(), pool, imsi, idx+1)
			if err != nil {
				errs[idx] = err
			} else {
				results[idx] = addr.String()
			}
		}(i)
	}

	wg.Wait()

	// All should succeed.
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d failed: %v", i, err)
		}
	}

	// All addresses should be unique.
	seen := make(map[string]bool)
	for i, addr := range results {
		if seen[addr] {
			t.Fatalf("goroutine %d got duplicate address %s", i, addr)
		}

		seen[addr] = true
	}
}

func TestAllocate_MergeScanCorrectness(t *testing.T) {
	store := newFakeStore()
	alloc := NewSequentialAllocator(store)
	pool := mustPool("10.0.0.0/29") // 6 usable: .1 through .6

	// Manually allocate .1, .3, .5 (with gaps at .2, .4, .6).
	seedSessIDs := []int{100, 101, 102}
	for i, addrSuffix := range []string{"10.0.0.1", "10.0.0.3", "10.0.0.5"} {
		sess := seedSessIDs[i]

		if err := store.CreateLease(context.Background(), &Lease{
			PoolID:    pool.ID,
			Address:   addrSuffix,
			IMSI:      "imsi-" + addrSuffix,
			SessionID: &sess,
			Type:      "dynamic",
			CreatedAt: 1000,
		}); err != nil {
			t.Fatalf("seed lease %s: %v", addrSuffix, err)
		}
	}

	// Next allocation should fill the first gap at .2.
	addr, err := alloc.Allocate(context.Background(), pool, "001010000000010", 10)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	if addr.String() != "10.0.0.2" {
		t.Fatalf("expected 10.0.0.2, got %s", addr)
	}

	// Next should fill .4.
	addr, err = alloc.Allocate(context.Background(), pool, "001010000000011", 11)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	if addr.String() != "10.0.0.4" {
		t.Fatalf("expected 10.0.0.4, got %s", addr)
	}

	// Next should fill .6.
	addr, err = alloc.Allocate(context.Background(), pool, "001010000000012", 12)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	if addr.String() != "10.0.0.6" {
		t.Fatalf("expected 10.0.0.6, got %s", addr)
	}

	// Pool should now be exhausted.
	_, err = alloc.Allocate(context.Background(), pool, "001010000000013", 13)
	if err != ErrPoolExhausted {
		t.Fatalf("expected ErrPoolExhausted, got %v", err)
	}
}
