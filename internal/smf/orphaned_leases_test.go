// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func (f *fakeStore) releasedIPv4Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.releasedIPs)
}

func failUPFDelete(t *testing.T, s *smf.SMF, upf *fakeUPF, store *fakeStore, ref string) {
	t.Helper()

	upf.err = errors.New("upf delete failed")

	if err := s.ReleaseSmContext(context.Background(), ref, models.ReleaseFor(models.ReleaseUERequested)); err == nil {
		t.Fatal("release succeeded although the UPF deletion failed")
	}

	upf.err = nil

	if got := store.releasedIPv4Count(); got != 0 {
		t.Fatalf("released %d leases although the UPF deletion failed", got)
	}
}

func TestUserPlaneSessionCleanedReleasesTheKeptLease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	seid := smCtx.PFCPContext.SEID

	failUPFDelete(t, s, upf, store, ref)

	s.UserPlaneSessionCleaned(context.Background(), seid)

	if got := store.releasedIPv4Count(); got != 1 {
		t.Fatalf("released %d leases after the UPF cleanup, want 1", got)
	}

	s.UserPlaneSessionCleaned(context.Background(), seid)

	if got := store.releasedIPv4Count(); got != 1 {
		t.Fatalf("released %d leases after a repeated report, want 1", got)
	}
}

func TestUserPlaneSessionCleanedKeepsALeaseACurrentSessionHolds(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	seid := smCtx.PFCPContext.SEID

	failUPFDelete(t, s, upf, store, ref)

	if _, err := s.NewSession(testSUPI(), smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	s.UserPlaneSessionCleaned(context.Background(), seid)

	if got := store.releasedIPv4Count(); got != 0 {
		t.Fatalf("released %d leases held by a current session", got)
	}
}

func TestReleaseTreatsAMissingUPFSessionAsDeleted(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	upf.err = fmt.Errorf("%w: SEID 100", models.ErrSessionNotFound)

	if err := s.ReleaseSmContext(context.Background(), ref, models.ReleaseFor(models.ReleaseUERequested)); err != nil {
		t.Fatalf("release of a session the UPF already removed = %v", err)
	}

	if got := store.releasedIPv4Count(); got != 1 {
		t.Fatalf("released %d leases, want 1 once the UPF session is gone", got)
	}
}
