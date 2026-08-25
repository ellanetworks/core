// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func framedTestPrefixes(t *testing.T, cidrs ...string) []netip.Prefix {
	t.Helper()

	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}

	return out
}

// TestCreateSessionEmitsFramedRoutes drives the shared establishment path (via
// the 4G entry) and asserts the resolved framed routes reach the UPF establish
// request (TS 23.501 §5.6.14, TS 29.244 §5.16).
func TestCreateSessionEmitsFramedRoutes(t *testing.T) {
	store, upf := epsTestSMF()
	store.framedRoutes = framedTestPrefixes(t, "192.168.10.0/24", "2001:db8:aa::/48")

	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(3)); err != nil {
		t.Fatal(err)
	}

	if upf.lastEstablish == nil {
		t.Fatal("no UPF establish request captured")
	}

	got := upf.lastEstablish.FramedRoutes
	if len(got) != 2 {
		t.Fatalf("expected 2 framed routes in establish request, got %+v", got)
	}
}

// TestCreateSessionFramedRouteResolveFailsEstablishment confirms a framed-route
// resolution error rejects the session (fail-closed, §5.4).
func TestCreateSessionFramedRouteResolveFailsEstablishment(t *testing.T) {
	store, upf := epsTestSMF()
	store.framedRoutesErr = errors.New("db unavailable")

	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err == nil {
		t.Fatal("expected establishment to fail when framed-route resolution fails")
	}
}

// TestFramedRoutesChanged covers the reconcile comparison: an identical set (even
// reordered) is unchanged; a different set is a change; an unknown session reports
// no change.
func TestFramedRoutesChanged(t *testing.T) {
	store, upf := epsTestSMF()
	store.framedRoutes = framedTestPrefixes(t, "192.168.10.0/24", "192.168.11.0/24")

	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatal(err)
	}

	store.framedRoutes = framedTestPrefixes(t, "192.168.11.0/24", "192.168.10.0/24")

	changed, err := framedRoutesChanged(s, bearer.Ref)
	if err != nil {
		t.Fatal(err)
	}

	if changed {
		t.Fatal("reordered identical framed-route set reported as changed")
	}

	store.framedRoutes = framedTestPrefixes(t, "192.168.10.0/24")

	changed, err = framedRoutesChanged(s, bearer.Ref)
	if err != nil {
		t.Fatal(err)
	}

	if !changed {
		t.Fatal("a changed framed-route set was not detected")
	}

	changed, err = framedRoutesChanged(s, "no-such-session")
	if err != nil {
		t.Fatal(err)
	}

	if changed {
		t.Fatal("unknown session reported a framed-route change")
	}
}

func framedRoutesChanged(s *smf.SMF, ref string) (bool, error) {
	delta, err := s.EPSSubscriptionChanged(context.Background(), ref)

	return delta.FramedRoutes, err
}

func TestEPSSubscriptionChangedResolvesDNNOnce(t *testing.T) {
	store, upf := epsTestSMF()
	store.framedRoutes = framedTestPrefixes(t, "192.168.10.0/24")
	store.staticIPv4 = store.allocatedIP

	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatal(err)
	}

	before := store.dnnResolves()

	delta, err := s.EPSSubscriptionChanged(context.Background(), bearer.Ref)
	if err != nil {
		t.Fatal(err)
	}

	if delta != (models.SubscriptionDelta{}) {
		t.Fatalf("subscription delta = %+v, want no change", delta)
	}

	if got := store.dnnResolves() - before; got != 1 {
		t.Fatalf("the subscription check resolved the data network %d times, want 1", got)
	}
}
