// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/core/sctp"
)

// TS 23.401 §5.3.5
func TestReleaseUEContextAfterAnEarlierReleaseCompleted(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	if len(cc.sent) != 1 {
		t.Fatalf("first release sent %d messages, want 1 UE Context Release Command", len(cc.sent))
	}

	m.FreeUeConn(t.Context(), ue)

	second := &captureConn{}
	m.AttachUeConn(t.Context(), ue, m.NewUeConn(second, 8))

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	if len(second.sent) != 1 {
		t.Fatalf("second release sent %d messages, want 1 UE Context Release Command: the claim from the first release outlived the connection it claimed, so nothing is ever commanded again", len(second.sent))
	}
}

func TestReleaseUEContextOnAConnectionThatSupersededOneStillReleasing(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	second := &captureConn{}
	m.AttachUeConn(t.Context(), ue, m.NewUeConn(second, 8))

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	if len(second.sent) != 1 {
		t.Fatalf("release on the resumed connection sent %d messages, want 1 UE Context Release Command", len(second.sent))
	}
}

// TS 23.401 §5.3.5: the release of a superseded S1 connection buffers the downlink of
// the bearers it was carrying, so data arriving meanwhile pages the UE instead of being
// forwarded to an eNB that is releasing it.
func TestAttachUeConn_DeactivatesTheSupersededConnectionsUserPlane(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"

	fake := m.Session.(*fakeSessionManager)
	fake.deactivated = false

	second := m.NewUeConn(&captureConn{}, 9)
	m.AttachUeConn(t.Context(), ue, second)

	if !fake.deactivated {
		t.Error("the superseded connection's user plane was left pointing at a released eNB context")
	}
}

type failingConn struct{}

func (failingConn) WriteMsg([]byte, *sctp.SndRcvInfo) (int, error) {
	return 0, errors.New("association down")
}

func (c failingConn) WriteMsgWait(_ context.Context, b []byte, info *sctp.SndRcvInfo) (int, error) {
	return c.WriteMsg(b, info)
}

func TestReleaseUEContextReleasesLocallyWhenTheCommandCannotBeSent(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	unreachable := m.NewUeConn(failingConn{}, 8)
	m.AttachUeConn(t.Context(), ue, unreachable)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	if ue.Connected() {
		t.Fatal("the UE stays ECM-CONNECTED on a connection its release command could not reach; it waits out the release guard")
	}
}

func TestUnansweredInitialContextSetupReleasesTheConnection(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	m.icsGuardCfg.ExpireTime = 10 * time.Millisecond

	c := ue.Conn()
	c.SetICS(ICSPending)
	c.SuperviseICS(t.Context())

	eventually(t, time.Second, func() bool { return cc.count() > 0 })
}

func TestAnsweredInitialContextSetupStopsItsSupervision(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	c := ue.Conn()
	c.SetICS(ICSPending)
	c.SuperviseICS(t.Context())

	if !c.icsGuard.Active() {
		t.Fatal("Initial Context Setup is not supervised")
	}

	c.SetICS(ICSCompleted)

	if c.icsGuard.Active() {
		t.Fatal("an answered Initial Context Setup is still supervised")
	}
}

func TestFallbackLocalReleaseSparesAConnectionThatWasAlreadyReplaced(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	failed := ue.Conn()
	current := m.NewUeConn(&captureConn{}, 8)
	m.AttachUeConn(t.Context(), ue, current)

	m.releaseUEContextLocally(t.Context(), ue, failed, "release-command-not-sent", models.ReleaseFor(models.ReleaseRegistrationFailure))

	if ue.Conn() != current {
		t.Fatal("the fallback release of a connection that was already replaced released the UE's current connection")
	}
}

func TestReleaseCompleteAfterTheReleaseGuardCleanedUpLeavesTheNextConnection(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	m.ReleaseUEContext(t.Context(), ue, CauseNASNormalRelease)

	released := ue.Conn()

	m.releaseUEContextLocally(t.Context(), ue, released, "release-command-timeout", models.ReleaseFor(models.ReleaseRegistrationFailure))

	next := m.NewUeConn(&captureConn{}, 8)
	m.AttachUeConn(t.Context(), ue, next)

	m.CompleteUEContextRelease(t.Context(), ue, released)

	if ue.Conn() != next {
		t.Fatal("a Release Complete for a connection the release guard already cleaned up released the UE's next connection")
	}

	if ue.MobileReachableArmedForTest() {
		t.Fatal("a Release Complete for a connection the release guard already cleaned up re-ran the idle transition")
	}
}

func TestAPIDChecksThatMatchNothingTakeOnlyTheReadLock(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	enbID := s1ap.ENBUES1APID(7)

	m.mu.RLock()
	defer m.mu.RUnlock()

	done := make(chan struct{})

	go func() {
		defer close(done)

		if !m.ClaimENBUES1APID(context.Background(), cc, enbID) {
			t.Error("a free eNB-UE-S1AP-ID was not claimable")
		}

		if m.ReleaseConnsWithAPIDs(context.Background(), cc, nil, &enbID, "test") {
			t.Error("released connections where none held the AP ID")
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("an AP ID check that matched no connection waited for the MME write lock")
	}
}

func TestDetachedReleaseGuardReleasesOnlyTheConnectionItGuards(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newTestMME(t)
		cc := &captureConn{}

		stale := m.NewUeConn(cc, 7)
		id := uint32(stale.MMEUES1APID)

		if !m.ReleaseDetachedConn(cc, stale.MMEUES1APID, 7) {
			t.Fatal("could not release the first connection")
		}

		m.guardDetachedRelease(t.Context(), stale)

		next := m.NewUeConn(cc, 7)

		m.mu.Lock()
		delete(m.conns, uint32(next.MMEUES1APID))
		next.MMEUES1APID = stale.MMEUES1APID
		m.conns[id] = next
		m.mu.Unlock()

		time.Sleep(time.Minute)
		synctest.Wait()

		m.mu.RLock()
		held := m.conns[id]
		m.mu.RUnlock()

		if held != next {
			t.Fatal("a release guard armed for a freed connection released a newer connection holding the same MME-UE-S1AP-ID and eNB-UE-S1AP-ID")
		}
	})
}
