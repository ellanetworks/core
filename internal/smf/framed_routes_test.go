// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
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

	imsi := epsRequest(1).IMSI
	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err != nil {
		t.Fatal(err)
	}

	store.framedRoutes = framedTestPrefixes(t, "192.168.11.0/24", "192.168.10.0/24")

	changed, err := s.FramedRoutesChanged(context.Background(), imsi, epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if changed {
		t.Fatal("reordered identical framed-route set reported as changed")
	}

	store.framedRoutes = framedTestPrefixes(t, "192.168.10.0/24")

	changed, err = s.FramedRoutesChanged(context.Background(), imsi, epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if !changed {
		t.Fatal("a changed framed-route set was not detected")
	}

	changed, err = s.FramedRoutesChanged(context.Background(), "001010000000009", epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if changed {
		t.Fatal("unknown session reported a framed-route change")
	}
}
