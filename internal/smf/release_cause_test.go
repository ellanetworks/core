// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
)

func buildPDUSessionReleaseRequestWithCause(pduSessionID, pti uint8, cause fgs.GSMCause) []byte {
	return []byte{uint8(fgs.EPD5GSM), pduSessionID, pti, uint8(fgs.MsgPDUSessionReleaseRequest), 0x59, uint8(cause)}
}

func equalRanNas(a, b models.RanNasRelCause) bool {
	eq := func(x, y *uint8) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }

	return eq(a.FiveGSm, b.FiveGSm) && eq(a.FiveGMm, b.FiveGMm) &&
		(a.NgAp == nil) == (b.NgAp == nil) && (a.NgAp == nil || *a.NgAp == *b.NgAp) &&
		(a.EPS == nil) == (b.EPS == nil) && (a.EPS == nil || *a.EPS == *b.EPS)
}

func assertReleased(t *testing.T, s *smf.SMF, sc *smf.SMContext, want models.SessionReleaseRequest) {
	t.Helper()

	if s.GetSession(sc.Ref) != nil {
		t.Fatal("session still pooled")
	}

	got := sc.ReleaseForTest()
	if got.Reason != want.Reason {
		t.Fatalf("release reason = %s, want %s", got.Reason, want.Reason)
	}

	if (got.SentCause == nil) != (want.SentCause == nil) || (got.SentCause != nil && *got.SentCause != *want.SentCause) {
		t.Fatalf("sent cause = %v, want %v", got.SentCause, want.SentCause)
	}

	if len(got.RanNas) != len(want.RanNas) {
		t.Fatalf("received causes = %+v, want %+v", got.RanNas, want.RanNas)
	}

	for i := range want.RanNas {
		if !equalRanNas(got.RanNas[i], want.RanNas[i]) {
			t.Fatalf("received cause %d = %+v, want %+v", i, got.RanNas[i], want.RanNas[i])
		}
	}
}

func completeRelease(t *testing.T, s *smf.SMF, sc *smf.SMContext, pti uint8) {
	t.Helper()

	ctx := context.Background()

	if _, err := s.UpdateSmContextN1Msg(ctx, sc.Ref, buildPDUSessionReleaseComplete(sc.PDUSessionID, pti)); err != nil {
		t.Fatalf("release complete: %v", err)
	}

	if _, err := s.UpdateSmContextN2InfoPduResRelRsp(ctx, sc.Ref); err != nil {
		t.Fatalf("N2 release response: %v", err)
	}
}

func TestUERequestedReleaseCarriesTheUEsCause(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionReleaseRequestWithCause(sc.PDUSessionID, 5, fgs.GSMCauseRegularDeactivation)); err != nil {
		t.Fatalf("release request: %v", err)
	}

	completeRelease(t, s, sc, 5)

	assertReleased(t, s, sc, models.ReleaseFor(models.ReleaseUERequested).
		WithRanNas(models.Received5GSMCause(uint8(fgs.GSMCauseRegularDeactivation))).
		WithSentCause(uint8(fgs.GSMCauseRegularDeactivation)))
}

func TestTheFirstReleaseCauseWins(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(sc.PDUSessionID, 5)); err != nil {
		t.Fatalf("release request: %v", err)
	}

	if err := s.ReleaseSmContext(ctx, ref, models.ReleaseFor(models.ReleaseDeregistration)); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	assertReleased(t, s, sc, models.ReleaseFor(models.ReleaseUERequested).WithSentCause(uint8(fgs.GSMCauseRegularDeactivation)))
}

func TestReleaseOnUEUnresponsiveKeepsTheTriggeringCause(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithT3592(procedureTimerInterval))

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionReleaseRequest(sc.PDUSessionID, 5)); err != nil {
		t.Fatalf("release request: %v", err)
	}

	waitFor(t, "session local release after T3592 expiry", func() bool { return s.GetSession(ref) == nil })

	assertReleased(t, s, sc, models.ReleaseFor(models.ReleaseUERequested).WithSentCause(uint8(fgs.GSMCauseRegularDeactivation)))
}

func TestASliceChangeReleaseIsAManagementIntervention(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	amfCb.err = smf.ErrUENotReachable
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, ref := setupSessionWithTunnel(t, s)

	if err := reconcileSliceMismatch(context.Background(), s, pcf, ref); err != nil {
		t.Fatalf("ReconcileSession: %v", err)
	}

	want := models.ReleaseFor(models.ReleasePolicyChanged).WithSentCause(uint8(fgs.GSMCauseReactivationRequested))
	assertReleased(t, s, sc, want)

	if got := want.Reason.ChargingTrigger(); got != models.ChargingTriggerManagementIntervention {
		t.Fatalf("charging trigger = %s, want management intervention", got)
	}
}

func TestAnEPSPolicyChangeRecordsNothingUntilTheMMEReleases(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	mme := &fakeMME{}
	s.SetMME(mme)

	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)

	if err := reconcileSliceMismatch(ctx, s, pcf, sc.Ref); err != nil {
		t.Fatalf("ReconcileSession: %v", err)
	}

	mme.mu.Lock()
	reasons := slices.Clone(mme.reactivationReasons)
	mme.mu.Unlock()

	if len(reasons) != 1 || reasons[0] != models.ReleasePolicyChanged {
		t.Fatalf("reactivation reasons handed to the MME = %v, want [policy_changed]", reasons)
	}

	if got := sc.ReleaseForTest(); got.Reason != "" {
		t.Fatalf("release recorded before anything was released: %+v", got)
	}

	if err := s.ReleaseEPSSession(ctx, sc.Ref, models.ReleaseFor(models.ReleaseDeregistration)); err != nil {
		t.Fatalf("ReleaseEPSSession: %v", err)
	}

	assertReleased(t, s, sc, models.ReleaseFor(models.ReleaseDeregistration))
}

func Test5GSMStatusReleaseIsAProtocolError(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(sc.PDUSessionID, 0, fgs.GSMCauseInvalidPDUSessionIdentity)); err != nil {
		t.Fatalf("5GSM STATUS: %v", err)
	}

	assertReleased(t, s, sc, models.ReleaseFor(models.ReleaseProtocolError).WithRanNas(models.Received5GSMCause(uint8(fgs.GSMCauseInvalidPDUSessionIdentity))))
}

func TestDuplicatePDUSessionIDRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextCauseDuplicatePDUSessionID(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextCauseDuplicatePDUSessionID: %v", err)
	}

	if _, err := s.UpdateSmContextN2InfoPduResRelRsp(ctx, ref); err != nil {
		t.Fatalf("N2 release response: %v", err)
	}

	assertReleased(t, s, sc, models.ReleaseFor(models.ReleaseDuplicateSessionID))
}

func TestAReleaseIsCountedOnceUnderItsReason(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	before := smf.SessionReleasesForTest(metrics.RAT5G, models.ReleaseStatusMismatch)

	cause := models.ReleaseFor(models.ReleaseStatusMismatch)

	if err := s.ReleaseSmContext(ctx, ref, cause); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	if err := s.ReleaseSmContext(ctx, ref, cause); err != nil {
		t.Fatalf("second ReleaseSmContext: %v", err)
	}

	if got := smf.SessionReleasesForTest(metrics.RAT5G, models.ReleaseStatusMismatch) - before; got != 1 {
		t.Fatalf("releases counted = %v, want 1", got)
	}
}

func TestASessionThatNeverEstablishedIsNotCountedAsReleased(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	upf.err = errUPFDown
	s := newTestSMF(pcf, store, upf, amfCb)

	before := smf.SessionReleasesForTest(metrics.RAT4G, models.ReleaseEstablishmentFailure)

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err == nil {
		t.Fatal("CreateEPSSession succeeded with the UPF down")
	}

	if got := smf.SessionReleasesForTest(metrics.RAT4G, models.ReleaseEstablishmentFailure) - before; got != 0 {
		t.Fatalf("releases counted for a session that was never established = %v, want 0", got)
	}
}

var errUPFDown = errors.New("upf down")
