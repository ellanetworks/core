// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

type blockingDeactivateSmf struct {
	*deregisterTestSmf

	mu      sync.Mutex
	calls   int
	events  []string
	entered chan struct{}
	unblock chan struct{}
}

func newBlockingDeactivateSmf() *blockingDeactivateSmf {
	return &blockingDeactivateSmf{
		deregisterTestSmf: &deregisterTestSmf{},
		entered:           make(chan struct{}),
		unblock:           make(chan struct{}),
	}
}

func (s *blockingDeactivateSmf) DeactivateSmContext(_ context.Context, ref string, _ bool) error {
	s.mu.Lock()
	s.calls++
	first := s.calls == 1
	s.mu.Unlock()

	if first {
		close(s.entered)
		<-s.unblock
	}

	s.record("deactivate:" + ref)

	return nil
}

func (s *blockingDeactivateSmf) record(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)
}

func (s *blockingDeactivateSmf) snapshot() (int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls, append([]string(nil), s.events...)
}

func newRaceConn(t *testing.T, a *AMF, name string, ranUeNgapID int64) *UeConn {
	t.Helper()

	radio := &Radio{name: name, Conn: &downlinkOrderConn{wrote: make(chan struct{}, 64)}}
	radio.BindAMFForTest(a)

	conn, err := a.NewUeConn(radio, models.RanUeNgapID(ranUeNgapID))
	if err != nil {
		t.Fatalf("NewUeConn: %v", err)
	}

	return conn
}

func newRegisteredRaceUE(t *testing.T, s SmfSbi) (*AMF, *UeContext) {
	t.Helper()

	a := New(nil, nil, s)
	ue := NewUeContext()
	ue.ForceStateForTest(Registered)
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}

	return a, ue
}

func TestStaleReleaseDoesNotUndoANewerConnection(t *testing.T) {
	s := newBlockingDeactivateSmf()
	a, ue := newRegisteredRaceUE(t, s)

	connA := newRaceConn(t, a, "gnb-a", 1)
	a.AttachUeConn(t.Context(), ue, connA)
	connA.SetN2SessionActive(1)

	connB := newRaceConn(t, a, "gnb-b", 2)

	released := make(chan struct{})

	go func() {
		defer close(released)

		a.ReleaseUeConnServedBy(context.Background(), connA, nil)
	}()

	<-s.entered

	attached := make(chan struct{})

	go func() {
		defer close(attached)

		a.AttachUeConn(context.Background(), ue, connB)
		s.record("activate")
	}()

	select {
	case <-attached:
	case <-time.After(50 * time.Millisecond):
	}

	close(s.unblock)
	<-released
	<-attached

	if ue.Conn() != connB {
		t.Fatal("the UE is not on the connection it attached to last")
	}

	if ue.MobileReachableActiveForTest() || ue.implicitDeregistrationTimer.Active() {
		t.Fatal("idle-mode timers run while the UE has a NAS signalling connection (TS 24.501 §5.3.7)")
	}

	_, events := s.snapshot()
	if len(events) == 0 || events[len(events)-1] != "activate" {
		t.Fatalf("a stale release deactivated the user plane after the newer connection activated it: %v", events)
	}
}

func TestReleaseOfAConnectionThatNoLongerServesTheUELeavesTheUEAlone(t *testing.T) {
	s := newBlockingDeactivateSmf()
	close(s.unblock)

	a, ue := newRegisteredRaceUE(t, s)

	connA := newRaceConn(t, a, "gnb-a", 1)
	a.AttachUeConn(t.Context(), ue, connA)
	connA.SetN2SessionActive(1)
	connA.Release(t.Context())

	connB := newRaceConn(t, a, "gnb-b", 2)
	a.AttachUeConn(t.Context(), ue, connB)

	connA.SetReleaseAction(UeContextReleaseAbortRegistration)

	if a.ReleaseUeConnServedBy(t.Context(), connA, nil) {
		t.Fatal("the release of a connection that no longer serves the UE reported the UE idle")
	}

	if calls, _ := s.snapshot(); calls != 0 {
		t.Fatalf("the release deactivated %d sessions the UE's current connection carries", calls)
	}

	if ue.Conn() != connB {
		t.Fatal("the release detached the UE's current connection")
	}

	if ue.State() != Registered {
		t.Fatalf("the release acted on the UE context: state %v", ue.State())
	}

	if ue.MobileReachableActiveForTest() {
		t.Fatal("the release armed the mobile reachable timer on a connected UE")
	}
}

func TestIdleTimersNeverRunOnAConnectedUE(t *testing.T) {
	a, ue := newRegisteredRaceUE(t, &deregisterTestSmf{})

	conn := newRaceConn(t, a, "gnb-a", 1)
	a.AttachUeConn(t.Context(), ue, conn)

	a.StartMobileReachable(ue, nil)

	if ue.MobileReachableActiveForTest() {
		t.Fatal("the mobile reachable timer was armed while the UE has a NAS signalling connection")
	}

	a.mu.Lock()
	gen := ue.idleGen
	a.mu.Unlock()

	a.onMobileReachableExpiry(ue, gen)

	if ue.implicitDeregistrationTimer.Active() {
		t.Fatal("mobile reachable expiry armed implicit deregistration on a connected UE")
	}

	a.onImplicitDeregistrationExpiry(ue, gen)

	if ue.State() != Registered || ue.Conn() != conn {
		t.Fatal("implicit deregistration expiry deregistered a connected UE")
	}

	a.StartMobileReachable(ue, conn)

	if !ue.MobileReachableActiveForTest() {
		t.Fatal("releasing the UE's own connection must arm the mobile reachable timer")
	}
}

func TestImplicitDeregistrationDoesNotRemoveAConnectionAttachedMeanwhile(t *testing.T) {
	a, ue := newRegisteredRaceUE(t, &deregisterTestSmf{})

	a.StartMobileReachable(ue, nil)

	a.mu.Lock()
	gen := ue.idleGen
	a.mu.Unlock()

	connB := newRaceConn(t, a, "gnb-b", 2)

	ue.connMu.Lock()

	expired := make(chan struct{})

	go func() {
		defer close(expired)

		a.onImplicitDeregistrationExpiry(ue, gen)
	}()

	select {
	case <-expired:
	case <-time.After(50 * time.Millisecond):
	}

	a.mu.Lock()
	a.attachUeConnLocked(t.Context(), ue, connB)
	a.mu.Unlock()

	ue.connMu.Unlock()
	<-expired

	if ue.Conn() != connB {
		t.Fatal("implicit deregistration detached the connection the UE established meanwhile")
	}

	if a.FindUEByRanUeNgapID(connB.Radio(), connB.RanUeNgapID()) != connB {
		t.Fatal("implicit deregistration removed the connection the UE established meanwhile")
	}

	if ue.State() != Registered {
		t.Fatalf("implicit deregistration ran after the UE established a NAS signalling connection (TS 24.501 §5.3.7): state %v", ue.State())
	}
}

func TestReleaseFollowsAConnectionMovedToAFreshContext(t *testing.T) {
	a := New(nil, nil, &deregisterTestSmf{})

	old := NewUeContext()
	old.ForceStateForTest(Registered)

	fresh := NewUeContext()
	fresh.ForceStateForTest(RegistrationInitiated)

	conn := newRaceConn(t, a, "gnb-a", 1)
	a.AttachUeConn(t.Context(), old, conn)
	conn.SetReleaseAction(UeContextReleaseAbortRegistration)

	old.connMu.Lock()

	released := make(chan struct{})

	go func() {
		defer close(released)

		a.ReleaseUeConn(context.Background(), conn)
	}()

	select {
	case <-released:
	case <-time.After(50 * time.Millisecond):
	}

	a.mu.Lock()
	a.attachUeConnLocked(t.Context(), fresh, conn)
	a.mu.Unlock()

	old.connMu.Unlock()
	<-released

	if fresh.State() != Deregistered {
		t.Fatalf("the release skipped the release action of the context the connection moved to: state %v", fresh.State())
	}
}
