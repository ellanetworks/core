// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func TestLocalReleaseTellsTheAMF(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	amfCb.err = smf.ErrUENotReachable
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	if err := reconcileSliceMismatch(context.Background(), s, pcf, ref); err != nil {
		t.Fatal(err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("the unreachable UE's session was not released locally")
	}

	if got := amfCb.dropped(); len(got) != 1 || got[0].ref != ref {
		t.Fatalf("AMF told %+v, want the released session dropped so the next Service Request syncs the UE", got)
	}
}

func TestReleaseWithoutAUserPlaneCarriesNoN2(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	if err := s.DeactivateSmContext(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if err := reconcileSliceMismatch(context.Background(), s, pcf, ref); err != nil {
		t.Fatal(err)
	}

	amfCb.mu.Lock()
	calls := amfCb.releaseCalls
	amfCb.mu.Unlock()

	if len(calls) != 1 || calls[0].n2Transfer != nil {
		t.Fatalf("release calls = %+v, want one N1-only release for a session with no user plane", calls)
	}
}

func TestReconcileWaitsForUserPlaneActivation(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	if err := s.DeactivateSmContext(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ActivateSmContext(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	reconcileAmbrChange(t, s, pcf, ref)

	if got := modifyCallCount(amfCb); got != 0 {
		t.Fatalf("a session mid-activation was modified %d times; the gNB would keep the setup's QoS", got)
	}
}

func TestIdleARPChangeIsCommittedWithoutSignalling(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	if err := s.DeactivateSmContext(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	err := reconcileWithPolicy(context.Background(), s, pcf, ref, &policyChange{
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := modifyCallCount(amfCb); got != 0 {
		t.Fatalf("an ARP-only change paged or signalled the UE %d times; the UE holds no ARP", got)
	}

	smCtx.Mutex.Lock()
	defer smCtx.Mutex.Unlock()

	if smCtx.PolicyData.QosData.Arp == nil || smCtx.PolicyData.QosData.Arp.PriorityLevel != 5 {
		t.Fatalf("ARP = %+v, want the committed 5 for the next activation", smCtx.PolicyData.QosData.Arp)
	}
}

func TestReconcileRebindsTheSessionToANewPolicy(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	policy := currentPolicy(s, ref)
	policy.PolicyID = "replacement"

	pcf.mu.Lock()
	pcf.policy = policy
	pcf.mu.Unlock()

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	upf.mu.Lock()
	calls := upf.modifyCalls
	upf.mu.Unlock()

	if len(calls) != 1 || calls[0].PolicyID != "replacement" {
		t.Fatalf("UPF modifies = %+v, want one binding the session to the new policy", calls)
	}

	if got := modifyCallCount(amfCb); got != 0 {
		t.Fatalf("a UPF-only change signalled the UE %d times", got)
	}

	smCtx.Mutex.Lock()
	defer smCtx.Mutex.Unlock()

	if smCtx.PolicyData.PolicyID != "replacement" {
		t.Fatalf("policy = %q, want the replacement", smCtx.PolicyData.PolicyID)
	}
}

func TestFailedUPFCommitIsRetried(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, pcf, ref)

	upf.mu.Lock()
	upf.modifyErrBySEID = map[uint64]error{100: errors.New("upf unavailable")}
	upf.mu.Unlock()

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionModificationComplete(smCtx.PDUSessionID, 0)); err != nil {
		t.Fatalf("modification complete: %v", err)
	}

	smCtx.Mutex.Lock()
	dl := smCtx.PolicyData.Ambr.Downlink
	smCtx.Mutex.Unlock()

	if !dl.Equal(models.MustParseBitRate("600 Mbps")) {
		t.Fatalf("downlink = %s, want the 600 Mbps the UE accepted", dl)
	}

	upf.mu.Lock()
	upf.modifyErrBySEID = nil
	before := len(upf.modifyCalls)
	upf.mu.Unlock()

	if err := reconcileWithPolicy(context.Background(), s, pcf, ref, nil); err != nil {
		t.Fatal(err)
	}

	upf.mu.Lock()
	after := len(upf.modifyCalls)
	upf.mu.Unlock()

	if after != before+1 {
		t.Fatalf("UPF modifies after the retry = %d, want %d", after, before+1)
	}
}
