// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func idleUE(t *testing.T) *UeContext {
	t.Helper()

	_, ue, _ := idleUEWithSmf(t)

	return ue
}

func idleUEWithSmf(t *testing.T) (*AMF, *UeContext, *deregisterTestSmf) {
	t.Helper()

	a, ue, _, fakeSmf := registeredUE(t)

	return a, ue, fakeSmf
}

func pagedUE(t *testing.T) (*AMF, *UeContext, *deregisterTestSmf) {
	t.Helper()

	a, ue, fakeSmf := idleUEWithSmf(t)

	cause, err := ue.beginPaging(&MTRequest{
		Req: models.N1N2MessageTransferRequest{PduSessionID: 5, Arp: &models.Arp{PriorityLevel: 9}},
	})
	if err != nil {
		t.Fatalf("beginPaging: %v", err)
	}

	if cause != models.N1N2AttemptingToReachUE {
		t.Fatalf("cause = %s, want %s", cause, models.N1N2AttemptingToReachUE)
	}

	return a, ue, fakeSmf
}

func TestDeregisterFailsThePendingTransfer(t *testing.T) {
	_, ue, fakeSmf := pagedUE(t)

	ue.Deregister(context.Background())

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after Deregister, want Idle", state)
	}

	if got := fakeSmf.transferFailures; len(got) != 1 || got[0] != models.N1N2FailureCauseUnspecified {
		t.Errorf("transfer failures = %v, want one FAILURE_CAUSE_UNSPECIFIED: the requester is never told the transfer was dropped", got)
	}
}

func TestMoveToEPSFailsThePendingTransfer(t *testing.T) {
	a, ue, _, fakeSmf := registeredUE(t)

	if _, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 5}}); err != nil {
		t.Fatalf("beginPaging: %v", err)
	}

	a.CancelRegistration(context.Background(), ue.Supi())

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the move to EPS, want Idle", state)
	}

	if len(fakeSmf.transferFailures) != 1 {
		t.Errorf("transfer failures = %v, want one", fakeSmf.transferFailures)
	}
}

func TestSuspendRegistrationFailsThePendingTransfer(t *testing.T) {
	_, ue, _ := pagedUE(t)

	ue.SuspendRegistration(context.Background())

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after SuspendRegistration, want Idle", state)
	}
}

func TestPagingAnsweredMovesToDelivering(t *testing.T) {
	_, ue, _ := pagedUE(t)

	ue.PagingAnswered()

	if req := ue.PagingPending(); req == nil || req.Req.PduSessionID != 5 {
		t.Fatalf("pending = %+v, want the paged request still held for delivery", req)
	}

	if state := ue.PagingState(); state != PagingDelivering {
		t.Errorf("paging state = %s after the UE answered, want Delivering", state)
	}

	ue.PagingDelivered()

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after delivery, want Idle", state)
	}
}

func TestConnectionReleaseFailsADeliveringTransfer(t *testing.T) {
	a, ue, fakeSmf := pagedUE(t)

	radio := &Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(a)

	conn := NewUeConnForTest(radio, 1, 10, logger.AmfLog)
	a.AttachUeConn(ue, conn)

	ue.PagingAnswered()

	conn.Release()

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the connection was released mid-delivery, want Idle", state)
	}

	if got := fakeSmf.transferFailures; len(got) != 1 || got[0] != models.N1N2UENotResponding {
		t.Errorf("transfer failures = %v, want one UE_NOT_RESPONDING", got)
	}
}

func TestSameOrLowerPriorityTransferIsRejectedWhileAttempting(t *testing.T) {
	ue := idleUE(t)

	high := &models.Arp{PriorityLevel: 5}

	if _, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 5, Arp: high}}); err != nil {
		t.Fatalf("beginPaging: %v", err)
	}

	_, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 6, Arp: &models.Arp{PriorityLevel: 9}}})

	var rejected *models.N1N2MessageTransferError
	if !asTransferError(err, &rejected) {
		t.Fatalf("err = %v, want a transfer rejection (TS 23.502 4.2.3.3 step 3b)", err)
	}

	if rejected.Cause != models.N1N2ErrHigherPriorityRequestOngoing {
		t.Errorf("cause = %s, want %s", rejected.Cause, models.N1N2ErrHigherPriorityRequestOngoing)
	}

	if rejected.Detail.HighestPrioArp == nil || rejected.Detail.HighestPrioArp.PriorityLevel != 5 {
		t.Errorf("highestPrioArp = %+v, want the ARP being paged for (TS 29.518 6.1.6.2.32)", rejected.Detail.HighestPrioArp)
	}

	if pending := ue.PagingPending(); pending == nil || pending.Req.PduSessionID != 5 {
		t.Errorf("pending = %+v, want the original request undisplaced", pending)
	}
}

func TestHigherPriorityTransferReplacesThePendingOne(t *testing.T) {
	ue := idleUE(t)

	if _, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 5, Arp: &models.Arp{PriorityLevel: 9}}}); err != nil {
		t.Fatalf("beginPaging: %v", err)
	}

	if _, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 6, Arp: &models.Arp{PriorityLevel: 2}}}); err != nil {
		t.Fatalf("a higher-priority transfer was rejected: %v", err)
	}

	if pending := ue.PagingPending(); pending == nil || pending.Req.PduSessionID != 6 {
		t.Errorf("pending = %+v, want the higher-priority request (TS 23.502 4.2.3.3 step 4b)", pending)
	}
}

func TestAbandonPagingKeepsTheTransferWhenTheUEAnsweredTheLastRetransmission(t *testing.T) {
	a, ue, fakeSmf := pagedUE(t)

	radio := &Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(a)

	conn := NewUeConnForTest(radio, 1, 10, logger.AmfLog)
	a.AttachUeConn(ue, conn)

	ue.PagingAnswered()

	a.abandonPaging(ue)

	if state := ue.PagingState(); state != PagingDelivering {
		t.Errorf("paging state = %s after an abort that raced the UE answering, want Delivering", state)
	}

	if pending := ue.PagingPending(); pending == nil || pending.Req.PduSessionID != 5 {
		t.Errorf("pending = %+v, want the buffered transfer kept for a UE that answered", pending)
	}

	if got := fakeSmf.transferFailures; len(got) != 0 {
		t.Errorf("transfer failures = %v, want none: the UE is CM-CONNECTED (TS 23.502 4.2.3.3)", got)
	}
}

// TS 29.518 §5.2.2.3.2: the consumer whose transfer the AMF will not deliver is notified.
func TestDisplacedTransferIsReportedToItsConsumer(t *testing.T) {
	_, ue, fakeSmf := pagedUE(t)

	if _, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 6, Arp: &models.Arp{PriorityLevel: 2}}}); err != nil {
		t.Fatalf("a higher-priority transfer was rejected: %v", err)
	}

	if got := fakeSmf.transferFailures; len(got) != 1 || got[0] != models.N1N2FailureCauseUnspecified {
		t.Fatalf("transfer failures = %v, want one FAILURE_CAUSE_UNSPECIFIED for the displaced transfer", got)
	}

	if got := fakeSmf.failedSessions; len(got) != 1 || got[0] != 5 {
		t.Errorf("failures reported for sessions %v, want the displaced session 5", got)
	}
}

// TS 23.502 §4.2.3.3 step 3a: the SMF re-invokes the transfer for the session it is
// already being paged for, which replaces the request rather than failing it.
func TestReinvokedTransferForTheSameSessionIsNotAFailure(t *testing.T) {
	_, ue, fakeSmf := pagedUE(t)

	if _, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 5, Arp: &models.Arp{PriorityLevel: 2}}}); err != nil {
		t.Fatalf("a higher-priority transfer was rejected: %v", err)
	}

	if got := fakeSmf.transferFailures; len(got) != 0 {
		t.Errorf("transfer failures = %v, want none: the request replaces the one it re-invokes", got)
	}

	if pending := ue.PagingPending(); pending == nil || pending.Req.Arp == nil || pending.Req.Arp.PriorityLevel != 2 {
		t.Errorf("pending = %+v, want the re-invoked request with the higher priority", pending)
	}
}

func TestPagingAttemptFailedReleasesItsOwnRequest(t *testing.T) {
	_, ue, fakeSmf := pagedUE(t)

	attempt := ue.PagingPending()

	ue.PagingAttemptFailed(attempt, models.N1N2FailureCauseUnspecified)

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the paging it buffered for could not be sent, want Idle", state)
	}

	if pending := ue.PagingPending(); pending != nil {
		t.Errorf("pending = %+v, want the unpaged request released", pending)
	}

	if got := fakeSmf.transferFailures; len(got) != 1 || got[0] != models.N1N2FailureCauseUnspecified {
		t.Errorf("transfer failures = %v, want one FAILURE_CAUSE_UNSPECIFIED for the request that was never paged for", got)
	}
}

func TestPagingAttemptFailedLeavesADisplacingRequestAlone(t *testing.T) {
	_, ue, fakeSmf := pagedUE(t)

	attempt := ue.PagingPending()

	if _, err := ue.beginPaging(&MTRequest{Req: models.N1N2MessageTransferRequest{PduSessionID: 6, Arp: &models.Arp{PriorityLevel: 2}}}); err != nil {
		t.Fatalf("a higher-priority transfer was rejected: %v", err)
	}

	fakeSmf.transferFailures = nil

	ue.PagingAttemptFailed(attempt, models.N1N2FailureCauseUnspecified)

	if state := ue.PagingState(); state != PagingAttempting {
		t.Errorf("paging state = %s, want the displacing procedure still Attempting", state)
	}

	if pending := ue.PagingPending(); pending == nil || pending.Req.PduSessionID != 6 {
		t.Errorf("pending = %+v, want the displacing request untouched", pending)
	}

	if got := fakeSmf.transferFailures; len(got) != 0 {
		t.Errorf("transfer failures = %v, want none: the displaced request was already reported when it was displaced", got)
	}
}

func asTransferError(err error, target **models.N1N2MessageTransferError) bool {
	return errors.As(err, target)
}

// TS 23.502 §4.2.3.3: the UE answers the page by establishing a NAS signalling
// connection, whatever initial NAS message it carries.
func TestAttachingAConnectionAnswersThePage(t *testing.T) {
	a, ue, _ := pagedUE(t)

	radio := &Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(a)

	a.AttachUeConn(ue, NewUeConnForTest(radio, 1, 10, logger.AmfLog))

	if state := ue.PagingState(); state != PagingDelivering {
		t.Errorf("paging state = %s after the UE re-established its connection, want Delivering", state)
	}

	if ue.PagingActiveForTest() {
		t.Error("the paging supervision guard is still armed after the UE answered")
	}
}
