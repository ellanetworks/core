// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package drain

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

type fakeNF struct {
	mu        sync.Mutex
	eligible  bool
	offloaded int
	remaining int
}

func (f *fakeNF) SetEligible(_ context.Context, eligible bool) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.eligible = eligible

	return 1
}

func (f *fakeNF) Offload(_ context.Context, batch int) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := f.remaining
	if batch > 0 && n > batch {
		n = batch
	}

	f.remaining -= n
	f.offloaded += n

	return n
}

func (f *fakeNF) RemainingOffloadable() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.remaining
}

type fakeStore struct {
	mu      sync.Mutex
	members map[int]*db.ClusterMember
	self    int
	bgpOn   bool
}

func (s *fakeStore) NodeID() int          { return s.self }
func (s *fakeStore) ClusterEnabled() bool { return true }

func (s *fakeStore) IsBGPEnabled(context.Context) (bool, error) { return s.bgpOn, nil }

func (s *fakeStore) GetClusterMember(_ context.Context, id int) (*db.ClusterMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[id]
	if !ok {
		return nil, db.ErrNotFound
	}

	c := *m

	return &c, nil
}

func (s *fakeStore) ListClusterMembers(context.Context) ([]db.ClusterMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]db.ClusterMember, 0, len(s.members))
	for _, m := range s.members {
		out = append(out, *m)
	}

	return out, nil
}

func (s *fakeStore) SetDrainState(_ context.Context, id int, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.members[id].DrainState = state

	return nil
}

func (s *fakeStore) stateOf(id int) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.members[id].DrainState
}

func newStore(selfState string, peerStates ...string) *fakeStore {
	s := &fakeStore{self: 1, members: map[int]*db.ClusterMember{
		1: {NodeID: 1, DrainState: selfState, DrainUpdatedAt: time.Now().Unix()},
	}}

	for i, st := range peerStates {
		id := i + 2
		s.members[id] = &db.ClusterMember{NodeID: id, DrainState: st}
	}

	return s
}

func TestReconcileMakesADrainingNodeIneligible(t *testing.T) {
	nf := &fakeNF{eligible: true}

	New(newStore(db.DrainStateDraining, db.DrainStateActive), nil, nil, nf).Reconcile(context.Background())

	nf.mu.Lock()
	defer nf.mu.Unlock()

	if nf.eligible {
		t.Fatal("a draining node was left eligible")
	}
}

func TestReconcileRestoresEligibilityWhenActive(t *testing.T) {
	nf := &fakeNF{eligible: false}

	New(newStore(db.DrainStateActive, db.DrainStateActive), nil, nil, nf).Reconcile(context.Background())

	nf.mu.Lock()
	defer nf.mu.Unlock()

	if !nf.eligible {
		t.Fatal("an active node was left ineligible")
	}
}

func TestSweepOffloadsAndCompletesTheDrain(t *testing.T) {
	store := newStore(db.DrainStateDraining, db.DrainStateActive)
	nf := &fakeNF{remaining: offloadBatchSize + 3}

	r := New(store, nil, nil, nf)

	r.sweep(context.Background())

	if got := store.stateOf(1); got != db.DrainStateDraining {
		t.Fatalf("state = %s, want still draining while UEs remain", got)
	}

	r.sweep(context.Background())
	r.sweep(context.Background())

	if got := store.stateOf(1); got != db.DrainStateDrained {
		t.Fatalf("state = %s, want drained once the node is empty", got)
	}

	if nf.offloaded != offloadBatchSize+3 {
		t.Fatalf("off-loaded %d UEs, want %d", nf.offloaded, offloadBatchSize+3)
	}
}

func TestSweepIgnoresTheBatchBoundPastTheDeadline(t *testing.T) {
	store := newStore(db.DrainStateDraining, db.DrainStateActive)
	store.members[1].DrainUpdatedAt = time.Now().Add(-2 * time.Hour).Unix()

	nf := &fakeNF{remaining: offloadBatchSize * 5}

	New(store, nil, nil, nf).sweep(context.Background())

	if nf.remaining != 0 {
		t.Fatalf("%d UEs left after the deadline pass, want 0", nf.remaining)
	}
}

func TestSweepDoesNotOffloadWithNoActivePeer(t *testing.T) {
	store := newStore(db.DrainStateDraining, db.DrainStateDrained)
	nf := &fakeNF{remaining: 5}

	New(store, nil, nil, nf).sweep(context.Background())

	if nf.offloaded != 0 {
		t.Fatalf("off-loaded %d UEs with nowhere to send them", nf.offloaded)
	}

	if got := store.stateOf(1); got != db.DrainStateDraining {
		t.Fatalf("state = %s, want draining: the drain cannot complete", got)
	}
}

func TestSweepIgnoresANodeThatIsNotDraining(t *testing.T) {
	nf := &fakeNF{remaining: 5}

	New(newStore(db.DrainStateActive, db.DrainStateActive), nil, nil, nf).sweep(context.Background())

	if nf.offloaded != 0 {
		t.Fatalf("off-loaded %d UEs on an active node", nf.offloaded)
	}
}
