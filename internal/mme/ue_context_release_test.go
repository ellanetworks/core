// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/sctp"
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

	m.releaseUEContextLocally(t.Context(), ue, failed, "release-command-not-sent")

	if ue.Conn() != current {
		t.Fatal("the fallback release of a connection that was already replaced released the UE's current connection")
	}
}
