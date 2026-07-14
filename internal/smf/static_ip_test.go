// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

// TestStaticIPChanged covers the reconcile comparison used by the MME: a
// reservation identical to the one cached at establishment is unchanged; a
// repinned, deleted, or newly created reservation is a change; an unknown
// session reports no change.
func TestStaticIPChanged(t *testing.T) {
	store, upf := epsTestSMF()
	store.staticIPv4 = store.allocatedIP // session established on a pinned address

	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	imsi := epsRequest(1).IMSI
	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err != nil {
		t.Fatal(err)
	}

	changed, err := s.StaticIPChanged(context.Background(), imsi, epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if changed {
		t.Fatal("unchanged static reservation reported as changed")
	}

	// Repin to a different address.
	store.staticIPv4 = netip.AddrFrom4([4]byte{10, 45, 0, 8})

	changed, err = s.StaticIPChanged(context.Background(), imsi, epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if !changed {
		t.Fatal("a repinned static reservation was not detected")
	}

	// Delete the reservation.
	store.staticIPv4 = netip.Addr{}

	changed, err = s.StaticIPChanged(context.Background(), imsi, epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if !changed {
		t.Fatal("a deleted static reservation was not detected")
	}

	changed, err = s.StaticIPChanged(context.Background(), "001010000000009", epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if changed {
		t.Fatal("unknown session reported a static IP change")
	}
}

// TestStaticIPChangedCreatedOnDynamicSession covers the reported bug: a session
// established on a dynamic address must be reported as changed when the operator
// later pins a static IP, so the reconciler releases it for re-establishment.
func TestStaticIPChangedCreatedOnDynamicSession(t *testing.T) {
	store, upf := epsTestSMF() // no static reservation at establishment

	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	imsi := epsRequest(1).IMSI
	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err != nil {
		t.Fatal(err)
	}

	changed, err := s.StaticIPChanged(context.Background(), imsi, epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if changed {
		t.Fatal("dynamic session with no reservation reported as changed")
	}

	// Operator pins a static IP mid-session.
	store.staticIPv4 = netip.AddrFrom4([4]byte{10, 45, 0, 9})

	changed, err = s.StaticIPChanged(context.Background(), imsi, epsTestEBI)
	if err != nil {
		t.Fatal(err)
	}

	if !changed {
		t.Fatal("a static IP created on a dynamic session was not detected")
	}
}

// TestReconcileSmContext_StaticIPCreatedReleasesSession asserts the 5G reconcile
// path releases the session (cause #39) when a static reservation is created for
// a session currently on a dynamic address.
func TestReconcileSmContext_StaticIPCreatedReleasesSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	// The session was set up on a dynamic address (10.0.0.1); the operator now
	// pins that subscriber to a static IP.
	store.mu.Lock()
	store.staticIPv4 = netip.MustParseAddr("10.0.0.1")
	store.mu.Unlock()

	if err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
	}); err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	amfCb.mu.Lock()
	releaseCalls := len(amfCb.releaseCalls)
	amfCb.mu.Unlock()

	if releaseCalls != 1 {
		t.Fatalf("expected 1 release signaling call after static IP change, got %d", releaseCalls)
	}
}
