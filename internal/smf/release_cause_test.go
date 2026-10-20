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
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func buildPDUSessionReleaseRequestWithCause(pduSessionID, pti uint8, cause fgs.GSMCause) []byte {
	return []byte{uint8(fgs.EPD5GSM), pduSessionID, pti, uint8(fgs.MsgPDUSessionReleaseRequest), 0x59, uint8(cause)}
}

func assertReleased(t *testing.T, s *smf.SMF, sc *smf.SMContext, want models.SessionReleaseCause) {
	t.Helper()

	if s.GetSession(sc.Ref) != nil {
		t.Fatal("session still pooled")
	}

	got := sc.ReleaseCauseForTest()
	if got.Reason != want.Reason || !slices.Equal(got.Diagnostics, want.Diagnostics) {
		t.Fatalf("release cause = %+v, want %+v", got, want)
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

	assertReleased(t, s, sc, models.ReleaseCause(models.ReleaseUERequested, models.GSMCause(uint8(fgs.GSMCauseRegularDeactivation))))
}

func TestTheFirstReleaseCauseWins(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(sc.PDUSessionID, 5)); err != nil {
		t.Fatalf("release request: %v", err)
	}

	if err := s.ReleaseSmContext(ctx, ref, models.ReleaseCause(models.ReleaseDeregistration)); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	assertReleased(t, s, sc, models.ReleaseCause(models.ReleaseUERequested))
}

func TestReleaseOnUEUnresponsiveKeepsTheTriggeringCause(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithT3592(procedureTimerInterval))

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionReleaseRequest(sc.PDUSessionID, 5)); err != nil {
		t.Fatalf("release request: %v", err)
	}

	waitFor(t, "session local release after T3592 expiry", func() bool { return s.GetSession(ref) == nil })

	assertReleased(t, s, sc, models.ReleaseCause(models.ReleaseUERequested))
}

func TestReactivationReleaseIsAManagementIntervention(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	amfCb.err = smf.ErrUENotReachable
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, ref := setupSessionWithTunnel(t, s)

	if err := reconcileSliceMismatch(context.Background(), s, pcf, ref); err != nil {
		t.Fatalf("ReconcileSession: %v", err)
	}

	want := models.ReleaseCause(models.ReleaseReactivationRequested, models.GSMCause(uint8(fgs.GSMCauseReactivationRequested)))
	assertReleased(t, s, sc, want)

	if got := want.Reason.RecordClosing(); got != models.RecordClosingManagementIntervention {
		t.Fatalf("record closing = %d, want management intervention", got)
	}
}

func TestEPSReactivationIsRecordedBeforeTheMMEReleases(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(&fakeMME{})

	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)

	if err := reconcileSliceMismatch(ctx, s, pcf, sc.Ref); err != nil {
		t.Fatalf("ReconcileSession: %v", err)
	}

	if err := s.ReleaseEPSSession(ctx, sc.Ref, models.ReleaseCause(models.ReleaseUERequested)); err != nil {
		t.Fatalf("ReleaseEPSSession: %v", err)
	}

	assertReleased(t, s, sc, models.ReleaseCause(models.ReleaseReactivationRequested, models.ESMCause(uint8(eps.ESMCauseReactivationRequested))))
}

func Test5GSMStatusReleaseIsAProtocolError(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(sc.PDUSessionID, 0, fgs.GSMCauseInvalidPDUSessionIdentity)); err != nil {
		t.Fatalf("5GSM STATUS: %v", err)
	}

	assertReleased(t, s, sc, models.ReleaseCause(models.ReleaseProtocolError, models.GSMCause(uint8(fgs.GSMCauseInvalidPDUSessionIdentity))))
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

	assertReleased(t, s, sc, models.ReleaseCause(models.ReleaseDuplicateSessionID))
}

func TestAReleaseIsCountedOnceUnderItsReason(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	before := smf.SessionReleasesForTest(metrics.RAT5G, models.ReleaseStatusMismatch)

	cause := models.ReleaseCause(models.ReleaseStatusMismatch)

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

func TestAFailedEstablishmentIsCountedAsSuch(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	upf.err = errUPFDown
	s := newTestSMF(pcf, store, upf, amfCb)

	before := smf.SessionReleasesForTest(metrics.RAT4G, models.ReleaseEstablishmentFailure)

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err == nil {
		t.Fatal("CreateEPSSession succeeded with the UPF down")
	}

	if got := smf.SessionReleasesForTest(metrics.RAT4G, models.ReleaseEstablishmentFailure) - before; got != 1 {
		t.Fatalf("establishment failures counted = %v, want 1", got)
	}
}

var errUPFDown = errors.New("upf down")
