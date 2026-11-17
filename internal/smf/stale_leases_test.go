// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
)

type fakeOwnedLeases struct {
	mu         sync.Mutex
	leases     []smf.OwnedLease
	released   []smf.OwnedLease
	releaseErr error
	entered    chan struct{}
	unblock    chan struct{}
}

func (f *fakeOwnedLeases) ListOwnedLeases(context.Context) ([]smf.OwnedLease, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.leases), nil
}

func (f *fakeOwnedLeases) ReleaseOwnedLease(_ context.Context, lease smf.OwnedLease) (netip.Addr, error) {
	if f.entered != nil {
		f.entered <- struct{}{}

		<-f.unblock
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.releaseErr != nil {
		return netip.Addr{}, f.releaseErr
	}

	f.released = append(f.released, lease)
	f.leases = slices.DeleteFunc(f.leases, func(l smf.OwnedLease) bool { return l == lease })

	return netip.MustParseAddr("10.0.0.1"), nil
}

func (f *fakeOwnedLeases) releasedLeases() []smf.OwnedLease {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.released)
}

func ownedLease(poolType string, sessionID int) smf.OwnedLease {
	return smf.OwnedLease{PoolID: "dn", PoolType: poolType, IMSI: testIMSI, SessionID: sessionID}
}

func TestReleaseStaleLeasesReleasesLeasesNoSessionHolds(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	leases := &fakeOwnedLeases{leases: []smf.OwnedLease{ownedLease("ipv4", 1), ownedLease("ipv6", 1)}}

	if got := s.ReleaseStaleLeases(context.Background(), leases); got != 2 {
		t.Fatalf("released %d leases, want 2", got)
	}

	if got := leases.releasedLeases(); !slices.Equal(got, []smf.OwnedLease{ownedLease("ipv4", 1), ownedLease("ipv6", 1)}) {
		t.Fatalf("released %+v", got)
	}
}

func TestReleaseStaleLeasesKeepsLeasesOfCurrentSessions(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	if _, err := s.NewSession(testSUPI(), smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if _, err := s.NewSession(testSUPI(), smf.Access4G, smf.SessionIdentity{EBI: 5}, testDNN, testSnssai); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	leases := &fakeOwnedLeases{leases: []smf.OwnedLease{ownedLease("ipv4", 1), ownedLease("ipv6", 1), ownedLease("ipv4", 64+5), ownedLease("ipv4", 2)}}

	if got := s.ReleaseStaleLeases(context.Background(), leases); got != 1 {
		t.Fatalf("released %d leases, want 1", got)
	}

	if got := leases.releasedLeases(); !slices.Equal(got, []smf.OwnedLease{ownedLease("ipv4", 2)}) {
		t.Fatalf("released %+v, want only the lease of session 2", got)
	}
}

func TestReleaseStaleLeasesSkipsUnknownIdentities(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	leases := &fakeOwnedLeases{leases: []smf.OwnedLease{
		ownedLease("ipv4", 0),
		ownedLease("ipv4", 256),
		{PoolID: "dn", PoolType: "ipv4", IMSI: "not-an-imsi", SessionID: 1},
	}}

	if got := s.ReleaseStaleLeases(context.Background(), leases); got != 0 {
		t.Fatalf("released %d leases with no valid session identity", got)
	}
}

func TestReleaseStaleLeasesReleasesALeaseWhoseSessionReleaseFailed(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	store.err = errors.New("raft commit timeout")

	if err := s.ReleaseSmContext(context.Background(), ref, models.ReleaseFor(models.ReleaseUERequested)); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	if s.HasPDUSession(testSUPI(), 1) {
		t.Fatal("session still held after its release")
	}

	leases := &fakeOwnedLeases{leases: []smf.OwnedLease{ownedLease("ipv4", 1)}}

	if got := s.ReleaseStaleLeases(context.Background(), leases); got != 1 {
		t.Fatalf("released %d leases, want the one the failed session release left", got)
	}
}

func TestReleaseStaleLeasesKeepsALeaseKeptForUserPlaneCleanup(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	failUPFDelete(t, s, upf, store, ref)

	leases := &fakeOwnedLeases{leases: []smf.OwnedLease{ownedLease("ipv4", 1)}}

	if got := s.ReleaseStaleLeases(context.Background(), leases); got != 0 {
		t.Fatalf("released %d leases kept until the user plane is cleaned", got)
	}
}

func TestReleaseStaleLeasesRetriesAfterAFailedRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	leases := &fakeOwnedLeases{
		leases:     []smf.OwnedLease{ownedLease("ipv4", 1), ownedLease("ipv4", 2)},
		releaseErr: errors.New("raft commit timeout"),
	}

	if got := s.ReleaseStaleLeases(context.Background(), leases); got != 0 {
		t.Fatalf("released %d leases while releases fail", got)
	}

	leases.mu.Lock()
	leases.releaseErr = nil
	leases.mu.Unlock()

	if got := s.ReleaseStaleLeases(context.Background(), leases); got != 2 {
		t.Fatalf("released %d leases on retry, want 2", got)
	}
}

func TestAllocationWaitsForAnInFlightStaleRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	leases := &fakeOwnedLeases{
		leases:  []smf.OwnedLease{ownedLease("ipv4", 1)},
		entered: make(chan struct{}),
		unblock: make(chan struct{}),
	}

	swept := make(chan int)

	go func() { swept <- s.ReleaseStaleLeases(context.Background(), leases) }()

	<-leases.entered

	sc, err := s.NewSession(testSUPI(), smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	sc.PDUSessionType = uint8(fgs.PDUSessionTypeIPv4)

	allocated := make(chan error)

	go func() { allocated <- s.AllocateUEAddressesForTest(context.Background(), sc) }()

	select {
	case err := <-allocated:
		t.Fatalf("allocation completed during the stale release: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	if got := store.allocSessionIDs(); len(got) != 0 {
		t.Fatalf("allocated %v during the stale release", got)
	}

	close(leases.unblock)

	if got := <-swept; got != 1 {
		t.Fatalf("released %d leases, want 1", got)
	}

	if err := <-allocated; err != nil {
		t.Fatalf("allocation after the stale release: %v", err)
	}

	if got := store.allocSessionIDs(); !slices.Equal(got, []uint8{1}) {
		t.Fatalf("allocated %v, want session 1", got)
	}
}

func TestAllocationGivesUpWaitingForAStaleReleaseWhenCancelled(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	leases := &fakeOwnedLeases{
		leases:  []smf.OwnedLease{ownedLease("ipv4", 1)},
		entered: make(chan struct{}),
		unblock: make(chan struct{}),
	}

	swept := make(chan int)

	go func() { swept <- s.ReleaseStaleLeases(context.Background(), leases) }()

	<-leases.entered

	sc, err := s.NewSession(testSUPI(), smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	sc.PDUSessionType = uint8(fgs.PDUSessionTypeIPv4)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := s.AllocateUEAddressesForTest(ctx, sc); !errors.Is(err, context.Canceled) {
		t.Fatalf("allocation = %v, want context.Canceled", err)
	}

	close(leases.unblock)
	<-swept

	if got := store.allocSessionIDs(); len(got) != 0 {
		t.Fatalf("allocated %v although the wait was cancelled", got)
	}
}

func TestRunStaleLeaseReleaserSweepsUntilCancelled(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	leases := &fakeOwnedLeases{leases: []smf.OwnedLease{ownedLease("ipv4", 1)}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		s.RunStaleLeaseReleaser(ctx, leases, time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for len(leases.releasedLeases()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the releaser never released the stale lease")
		}

		time.Sleep(time.Millisecond)
	}

	cancel()
	<-done
}
