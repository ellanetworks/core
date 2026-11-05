// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"golang.org/x/sys/unix"
)

func estRequest(t *testing.T, pduSessionID uint8) []byte {
	t.Helper()

	ipv4 := fgs.PDUSessionTypeIPv4

	buf, err := (&fgs.PDUSessionEstablishmentRequest{
		PDUSessionID:             fgs.PDUSessionID(pduSessionID),
		PTI:                      10,
		IntegrityProtMaxDataRate: [2]byte{0xff, 0xff},
		PDUSessionType:           &ipv4,
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("build PDU Session Establishment Request: %v", err)
	}

	return buf
}

func createSession(ctx context.Context, t *testing.T, s *smf.SMF, pduSessionID uint8, requestType fgs.RequestType) (string, *fgs.PDUSessionEstablishmentReject, error) {
	t.Helper()

	ref, rsp, err := s.CreateSmContext(ctx, testSUPI(), pduSessionID, testDNN, testSnssai, requestType, estRequest(t, pduSessionID), 0)
	if rsp == nil {
		return ref, nil, err
	}

	reject, perr := fgs.ParsePDUSessionEstablishmentReject(rsp)
	if perr != nil {
		t.Fatalf("decode PDU Session Establishment Reject: %v", perr)
	}

	return ref, reject, err
}

func fillToCapacity(ctx context.Context, t *testing.T, s *smf.SMF, n int) {
	t.Helper()

	for i := 1; i <= n; i++ {
		ref, reject, err := createSession(ctx, t, s, uint8(i), fgs.RequestTypeInitialRequest)
		if err != nil || reject != nil || ref == "" {
			t.Fatalf("session %d: ref %q, reject %+v, err %v", i, ref, reject, err)
		}
	}
}

func assertCapacityReject(t *testing.T, reject *fgs.PDUSessionEstablishmentReject, err error, wantBackoff bool) {
	t.Helper()

	if !errors.Is(err, smf.ErrDatapathCapacity) {
		t.Fatalf("err = %v, want ErrDatapathCapacity", err)
	}

	if reject == nil {
		t.Fatal("expected a PDU Session Establishment Reject")
	}

	if reject.Cause != fgs.GSMCauseInsufficientResources {
		t.Fatalf("5GSM cause = %d, want #26", reject.Cause)
	}

	if !wantBackoff {
		if reject.BackoffTimer != nil {
			t.Fatalf("back-off timer = %+v, want none", reject.BackoffTimer)
		}

		return
	}

	if reject.BackoffTimer == nil {
		t.Fatal("expected a Back-off timer value IE")
	}

	if d, ok := reject.BackoffTimer.Duration(); !ok || d != 60*time.Second {
		t.Fatalf("back-off = %v (%t), want 60s", d, ok)
	}
}

func TestCreateSmContextRejectsAtMaxSessions(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithMaxSessions(2))
	ctx := smf.WithHighPriorityAccess(context.Background(), false)

	fillToCapacity(ctx, t, s, 2)

	_, reject, err := createSession(ctx, t, s, 3, fgs.RequestTypeInitialRequest)
	assertCapacityReject(t, reject, err, true)

	if got := s.SessionCount(); got != 2 {
		t.Fatalf("SessionCount = %d, want 2", got)
	}
}

func TestCreateSmContextCapacityRejectOmitsBackoff(t *testing.T) {
	cases := []struct {
		name        string
		ctx         context.Context
		requestType fgs.RequestType
	}{
		{"high priority access", smf.WithHighPriorityAccess(context.Background(), true), fgs.RequestTypeInitialRequest},
		{"access priority unknown", context.Background(), fgs.RequestTypeInitialRequest},
		{"initial emergency request", smf.WithHighPriorityAccess(context.Background(), false), fgs.RequestTypeInitialEmergencyRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pcf, store, upf, amfCb := defaultFakes()
			s := newTestSMF(pcf, store, upf, amfCb, smf.WithMaxSessions(1))

			fillToCapacity(tc.ctx, t, s, 1)

			_, reject, err := createSession(tc.ctx, t, s, 2, tc.requestType)
			assertCapacityReject(t, reject, err, false)
		})
	}
}

func TestCreateSmContextAdmitsAfterRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithMaxSessions(1))
	ctx := smf.WithHighPriorityAccess(context.Background(), false)

	ref, _, err := createSession(ctx, t, s, 1, fgs.RequestTypeInitialRequest)
	if err != nil {
		t.Fatalf("first session: %v", err)
	}

	if _, reject, err := createSession(ctx, t, s, 2, fgs.RequestTypeInitialRequest); reject == nil || err == nil {
		t.Fatal("expected the second session to be rejected at capacity")
	}

	if err := s.ReleaseSmContext(ctx, ref, models.ReleaseFor(models.ReleaseUERequested)); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	if _, reject, err := createSession(ctx, t, s, 2, fgs.RequestTypeInitialRequest); err != nil || reject != nil {
		t.Fatalf("session after release: reject %+v, err %v", reject, err)
	}
}

func TestCreateSmContextWithoutMaxSessionsIsUnlimited(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	fillToCapacity(context.Background(), t, s, 15)
}

func TestCreateSmContextDatapathInsertFailure(t *testing.T) {
	cases := []struct {
		name      string
		upfErr    error
		wantCause fgs.GSMCause
		capacity  bool
	}{
		{"E2BIG", fmt.Errorf("couldn't apply PDR: can't apply GTP PDR: %w", fmt.Errorf("update: %w", unix.E2BIG)), fgs.GSMCauseInsufficientResources, true},
		{"ENOMEM", errors.Join(fmt.Errorf("can't put URR: %w", unix.ENOMEM), errors.New("cleanup")), fgs.GSMCauseInsufficientResources, true},
		{"other", errors.New("map is not initialized"), fgs.GSMCauseRequestRejectedUnspecified, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pcf, store, upf, amfCb := defaultFakes()
			upf.err = tc.upfErr
			s := newTestSMF(pcf, store, upf, amfCb)
			ctx := smf.WithHighPriorityAccess(context.Background(), false)

			_, reject, err := createSession(ctx, t, s, 1, fgs.RequestTypeInitialRequest)

			if tc.capacity {
				assertCapacityReject(t, reject, err, true)
				return
			}

			if errors.Is(err, smf.ErrDatapathCapacity) {
				t.Fatalf("err = %v, want no capacity error", err)
			}

			if reject == nil || reject.Cause != tc.wantCause || reject.BackoffTimer != nil {
				t.Fatalf("reject = %+v, want cause %d without back-off", reject, tc.wantCause)
			}
		})
	}
}

func TestCreateEPSSessionRejectsAtMaxSessions(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{policy: epsPolicy()}, store, upf, &fakeAMF{}, smf.WithMaxSessions(1))

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err != nil {
		t.Fatalf("first EPS session: %v", err)
	}

	req := epsRequest(1)
	req.EPSBearerIdentity = epsTestEBI + 1
	req.PDUSessionID = 2

	_, err := s.CreateEPSSession(context.Background(), req)
	if !errors.Is(err, smf.ErrDatapathCapacity) {
		t.Fatalf("err = %v, want ErrDatapathCapacity", err)
	}

	if got := s.SessionCount(); got != 1 {
		t.Fatalf("SessionCount = %d, want 1", got)
	}
}

func TestCreateEPSSessionCountsAcrossAccesses(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithMaxSessions(1))

	fillToCapacity(context.Background(), t, s, 1)

	req := epsRequest(1)
	req.IMSI = "001010000000002"

	if _, err := s.CreateEPSSession(context.Background(), req); !errors.Is(err, smf.ErrDatapathCapacity) {
		t.Fatalf("err = %v, want ErrDatapathCapacity", err)
	}
}

func TestCreateEPSSessionDatapathInsertFailure(t *testing.T) {
	store, upf := epsTestSMF()
	upf.err = fmt.Errorf("couldn't apply PDR: %w", unix.E2BIG)
	s := newTestSMF(&fakePCF{policy: epsPolicy()}, store, upf, &fakeAMF{})

	_, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if !errors.Is(err, smf.ErrDatapathCapacity) || !errors.Is(err, unix.E2BIG) {
		t.Fatalf("err = %v, want ErrDatapathCapacity wrapping E2BIG", err)
	}
}
