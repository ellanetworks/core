// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// A session's Ref and its PDU session identity both survive a move, so a
// per-access entry point keyed on either reaches a session the other access now
// serves. The EPS bearer identity does not survive, so the entry points keyed on
// it are unreachable once the move has committed and are not covered here.

func TestDeactivateEPSSessionRefusesASessionOnFiveGS(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := movedFromEPSTo5GS(t, s)

	upf.mu.Lock()
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	if err := s.DeactivateEPSSession(context.Background(), ref); err == nil {
		t.Error("DeactivateEPSSession(session on 5GS) = nil, want an error")
	}

	upf.mu.Lock()
	modifiesAfter := len(upf.modifyCalls)
	upf.mu.Unlock()

	if modifiesAfter != modifiesBefore {
		t.Errorf("UPF ModifySession calls = %d, want %d", modifiesAfter, modifiesBefore)
	}
}

func TestDeactivateSmContextRefusesASessionOnEPS(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	upf.mu.Lock()
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	if err := s.DeactivateSmContext(context.Background(), bearer.Ref); err == nil {
		t.Error("DeactivateSmContext(session on EPS) = nil, want an error")
	}

	upf.mu.Lock()
	modifiesAfter := len(upf.modifyCalls)
	upf.mu.Unlock()

	if modifiesAfter != modifiesBefore {
		t.Errorf("UPF ModifySession calls = %d, want %d", modifiesAfter, modifiesBefore)
	}
}

// The subscription checks answer the MME, which acts on a change by rebuilding
// the PDN connection. Reporting one for a session the MME no longer serves would
// have it tear down a live 5GS session.
func TestSubscriptionChangesAreReportedOnlyWhileTheSessionIsOnEPS(t *testing.T) {
	for _, tc := range []struct {
		name    string
		provoke func(*fakeStore)
		check   func(*smf.SMF, context.Context, string) (bool, error)
	}{
		{
			name: "framed routes",
			provoke: func(store *fakeStore) {
				store.mu.Lock()
				defer store.mu.Unlock()

				store.framedRoutes = []netip.Prefix{netip.MustParsePrefix("10.9.0.0/24")}
			},
			check: (*smf.SMF).FramedRoutesChanged,
		},
		{
			name: "static IP",
			provoke: func(store *fakeStore) {
				store.mu.Lock()
				defer store.mu.Unlock()

				store.staticIPv4 = netip.MustParseAddr("10.45.0.99")
			},
			check: (*smf.SMF).StaticIPChanged,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcf, store, upf, amfCb := defaultFakes()
			s := newTestSMF(pcf, store, upf, amfCb)
			ctx := context.Background()

			bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeInitialRequest))
			if err != nil {
				t.Fatalf("CreateEPSSession: %v", err)
			}

			tc.provoke(store)

			changed, err := tc.check(s, ctx, bearer.Ref)
			if err != nil {
				t.Fatalf("subscription check on EPS: %v", err)
			}

			if !changed {
				t.Fatal("change reported for a session on EPS = false, want true")
			}

			if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil {
				t.Fatalf("CreateSmContext(existing PDU session): %v", err)
			}

			changed, err = tc.check(s, ctx, bearer.Ref)
			if err != nil {
				t.Fatalf("subscription check on 5GS: %v", err)
			}

			if changed {
				t.Error("change reported for a session on 5GS = true, want false")
			}
		})
	}
}

// A 4G session carrying a PDU session identity is indexed under it, so the 5GS
// paging entry points resolve it and must refuse it.
func TestPagingFailureRefusesASessionOnEPS(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	if _, err := s.CreateEPSSession(context.Background(), epsTransferRequest(t, eps.RequestTypeInitialRequest)); err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if err := s.HandlePagingFailure(context.Background(), testSUPI(), transferTestPDUSessionID); err == nil {
		t.Error("HandlePagingFailure(session on EPS) = nil, want an error")
	}

	upf.mu.Lock()
	suppressed := len(upf.suppressDDNCalls)
	upf.mu.Unlock()

	if suppressed != 0 {
		t.Errorf("UPF downlink-notification suppressions = %d, want 0", suppressed)
	}
}

// Clearing is best-effort, so a session on the other access is skipped rather
// than reported as an error.
func TestClearPagingSuppressionSkipsASessionOnEPS(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	if _, err := s.CreateEPSSession(context.Background(), epsTransferRequest(t, eps.RequestTypeInitialRequest)); err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if err := s.ClearPagingSuppression(context.Background(), testSUPI(), transferTestPDUSessionID); err != nil {
		t.Errorf("ClearPagingSuppression(session on EPS) = %v, want nil", err)
	}

	upf.mu.Lock()
	cleared := len(upf.clearDDNCalls)
	upf.mu.Unlock()

	if cleared != 0 {
		t.Errorf("UPF downlink-notification clears = %d, want 0", cleared)
	}
}
