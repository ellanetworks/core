// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

type gatedSessionManager struct {
	*fakeSessionManager

	mu    sync.Mutex
	calls []string
	hook  func()
}

func (g *gatedSessionManager) record(call string) {
	g.mu.Lock()
	g.calls = append(g.calls, call)
	g.mu.Unlock()
}

func (g *gatedSessionManager) callLog() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return slices.Clone(g.calls)
}

func (g *gatedSessionManager) DeactivateEPSSession(ctx context.Context, ref string) error {
	g.mu.Lock()
	hook := g.hook
	g.hook = nil
	g.mu.Unlock()

	if hook != nil {
		hook()
	}

	g.record("deactivate")

	return g.fakeSessionManager.DeactivateEPSSession(ctx, ref)
}

func (g *gatedSessionManager) ModifyEPSSession(ctx context.Context, ref string, ebi uint8, enb models.FTEID, dedicated []models.DedicatedBearerEndpoint) error {
	g.record("modify")

	return g.fakeSessionManager.ModifyEPSSession(ctx, ref, ebi, enb, dedicated)
}

func (g *gatedSessionManager) runDuringNextDeactivation(t *testing.T, f func()) <-chan struct{} {
	t.Helper()

	done := make(chan struct{})

	g.mu.Lock()
	g.hook = func() {
		go func() {
			defer close(done)

			f()
		}()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
		}
	}
	g.mu.Unlock()

	return done
}

func newGatedMME(t *testing.T) (*MME, *gatedSessionManager) {
	t.Helper()

	g := &gatedSessionManager{fakeSessionManager: &fakeSessionManager{}}

	return New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, g), g
}

func connectedRegisteredUE(t *testing.T, m *MME) (*UeContext, *UeConn) {
	t.Helper()

	ue, _ := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	p := testPDN(ue)
	p.Apn = "internet"
	p.SessionRef = "ref-5"

	return ue, ue.Conn()
}

func awaitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the concurrent reattach never completed")
	}
}

func TestLocalReleaseDoesNotArmIdleTimersOnAUEThatReattachedMeanwhile(t *testing.T) {
	m, g := newGatedMME(t)
	ue, a := connectedRegisteredUE(t, m)

	b := m.NewUeConn(&captureConn{}, 9)
	done := g.runDuringNextDeactivation(t, func() { m.AttachUeConn(context.Background(), ue, b) })

	m.ReclaimConns(t.Context(), []*UeConn{a}, "eNB disconnect")
	awaitDone(t, done)

	if ue.Conn() != b {
		t.Fatal("the UE is not connected on the connection it reattached on")
	}

	if ue.MobileReachableArmedForTest() {
		t.Fatal("the mobile reachable timer runs while the UE holds a NAS signalling connection; it implicitly detaches a connected UE")
	}
}

func TestReleaseDeactivationDoesNotLandAfterTheReattachedConnectionsUserPlane(t *testing.T) {
	m, g := newGatedMME(t)
	ue, a := connectedRegisteredUE(t, m)

	b := m.NewUeConn(&captureConn{}, 9)
	done := g.runDuringNextDeactivation(t, func() {
		m.AttachUeConn(context.Background(), ue, b)
		m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{
			Present: []RANBearer{{Ebi: DefaultERABID, EnbFTEID: models.FTEID{TEID: 7, Addr: netip.MustParseAddr("10.0.0.9")}}},
			Conn:    b,
		})
	})

	m.ReleaseUEContext(t.Context(), ue, a, CauseNASNormalRelease)
	awaitDone(t, done)

	calls := g.callLog()
	if len(calls) == 0 || calls[len(calls)-1] != "modify" {
		t.Fatalf("anchor calls = %v, want the reattached connection's modify last: a release deactivation that lands after it black-holes the downlink of a connected UE", calls)
	}
}

func TestReleaseOfASupersededConnectionLeavesTheCurrentOneAlone(t *testing.T) {
	m := newTestMME(t)
	ue, a := connectedRegisteredUE(t, m)

	ccB := &captureConn{}
	b := m.NewUeConn(ccB, 9)
	m.AttachUeConn(t.Context(), ue, b)

	b.SetICS(ICSPending)
	b.DeferRelease(t.Context(), CauseNASNormalRelease)
	t.Cleanup(b.cancelDeferredRelease)

	fake := m.Session.(*fakeSessionManager)
	fake.deactivated = false

	m.ReleaseUEContext(t.Context(), ue, a, CauseNASUnspecified)

	if ue.Conn() != b {
		t.Fatal("the release of the superseded connection moved the UE off its current one")
	}

	if ccB.count() != 0 {
		t.Fatalf("the current connection received %d messages, want no UE Context Release Command", ccB.count())
	}

	if fake.deactivated {
		t.Fatal("the release of the superseded connection deactivated the current connection's user plane")
	}

	if b.deferredCause.Load() == nil {
		t.Fatal("the release of the superseded connection cancelled the current connection's deferred release")
	}
}

func TestICSResponseOfASupersededConnectionDoesNotMoveTheDownlink(t *testing.T) {
	m := newTestMME(t)
	ue, a := connectedRegisteredUE(t, m)

	m.AttachUeConn(t.Context(), ue, m.NewUeConn(&captureConn{}, 9))

	fake := m.Session.(*fakeSessionManager)
	fake.modifiedENB = models.FTEID{}

	result := m.ReconcileBearersToRAN(t.Context(), ue, RANBearers{
		Present: []RANBearer{{Ebi: DefaultERABID, EnbFTEID: models.FTEID{TEID: 7, Addr: netip.MustParseAddr("10.0.0.7")}}},
		Conn:    a,
	})

	if !result.Stale {
		t.Fatal("the reconciliation of a superseded connection was not reported stale")
	}

	if fake.modifiedENB != (models.FTEID{}) {
		t.Fatalf("the downlink was switched to %v, the eNB of a superseded connection", fake.modifiedENB)
	}
}

func TestIdleTimersNeverRunOnAConnectedUE(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedRegisteredUE(t, m)

	m.StartMobileReachable(ue)

	if ue.MobileReachableArmedForTest() {
		t.Fatal("the mobile reachable timer was armed on a UE with a NAS signalling connection")
	}

	m.mu.Lock()
	gen := ue.idleGen
	m.mu.Unlock()

	m.onMobileReachableExpiry(ue, gen)

	if ue.implicitDetachTimer.Active() {
		t.Fatal("the implicit detach timer was armed on a UE with a NAS signalling connection")
	}

	m.onImplicitDetachExpiry(ue, gen)

	if ue.EMMState() != EMMRegistered {
		t.Fatalf("a connected UE was implicitly detached: EMM state %v", ue.EMMState())
	}

	if m.Session.(*fakeSessionManager).wasReleased() {
		t.Fatal("a connected UE's sessions were released by the implicit detach timer")
	}
}

func TestPDNFilledAfterTeardownIsNotRecreated(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedRegisteredUE(t, m)

	p := m.AddPDN(ue)
	if p == nil {
		t.Fatal("no EPS bearer identity free")
	}

	m.ReleaseAllSessions(t.Context(), ue, models.ReleaseFor(models.ReleaseDeregistration))

	if got := m.FillBearer(ue, p, "ims", models.EPSBearer{Ref: "late"}); got != nil {
		t.Fatal("a PDN connection was recreated on a UE whose PDN connections were torn down")
	}

	if m.LookupPDN(ue, p.Ebi) != nil {
		t.Fatal("the torn-down UE holds a PDN connection again")
	}
}
