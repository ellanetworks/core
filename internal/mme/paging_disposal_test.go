// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/trace"
)

func TestPagingFailedReportsTheCauseForThePendingBearer(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	if state := ue.PagingState(); state != PagingAttempting {
		t.Fatalf("paging state = %s after Page, want Attempting", state)
	}

	dropped := ue.PagingFailed(t.Context(), models.EPSPagingUENotResponding)
	if dropped == nil || dropped.Ebi != 5 {
		t.Fatalf("dropped = %+v, want the pending bearer", dropped)
	}

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the failure, want Idle", state)
	}
}

func TestPagingAnsweredThenDelivered(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingAnswered()

	if state := ue.PagingState(); state != PagingDelivering {
		t.Errorf("paging state = %s after the UE answered, want Delivering", state)
	}

	ue.PagingDelivered(t.Context())

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after delivery, want Idle", state)
	}
}

func TestClearPagingDropsTheBufferedLPPa(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	ue.SetLPPaBuffered(7, []byte{0x01})

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingFailed(t.Context(), models.EPSPagingUENotResponding)

	if ue.PopLPPaBuffered() != nil {
		t.Error("the buffered LPPa payload survived the failed paging procedure")
	}
}

func TestDetachFailsThePendingTransfer(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.TransitionTo(t.Context(), EMMDeregistered)

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the UE was deregistered, want Idle", state)
	}
}

func TestConnectionReleaseFailsADeliveringTransfer(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingAnswered()

	m.ReleaseUEContextLocally(t.Context(), ue, "test")

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the connection carrying the delivery was released, want Idle", state)
	}
}

// TS 29.274 §7.2.11.3: no Downlink Data Notification Failure Indication after the
// MME successfully receives the Service Request from the UE.
func TestAbandonPagingKeepsTheTransferWhenTheUEAnsweredTheLastRetransmission(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.SetLPPaBuffered(7, []byte{0x01})

	ue.Pdns = map[uint8]*PdnConnection{
		5: {Ebi: 5},
		6: {Ebi: 6},
	}

	m.AttachUeConn(t.Context(), ue, m.NewUeConn(&captureConn{}, 9))

	m.abandonPaging(trace.SpanContext{}, ue, ue.paging.attempt)

	if state := ue.PagingState(); state != PagingDelivering {
		t.Errorf("paging state = %s after an abort that raced the UE answering, want Delivering", state)
	}

	if pending := ue.PagingPending(); pending == nil || pending.Ebi != 5 {
		t.Errorf("pending = %+v, want the paged bearer kept for delivery", pending)
	}

	if ue.PopLPPaBuffered() == nil {
		t.Error("the buffered LPPa payload was discarded although the UE answered the page")
	}

	if got := m.Session.(*fakeSessionManager).suppressCalls; got != 0 {
		t.Errorf("downlink data notification failures = %d, want 0 for a UE that answered", got)
	}
}

// TS 24.301 §5.5.3.2.2: a UE may answer paging with a tracking area update, which ends
// without an Initial Context Setup, so the release is what settles the transfer.
func TestReleaseCompleteFailsADeliveringTransfer(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	m.AttachUeConn(t.Context(), ue, m.NewUeConn(&captureConn{}, 9))

	if state := ue.PagingState(); state != PagingDelivering {
		t.Fatalf("paging state = %s after the UE answered, want Delivering", state)
	}

	m.FreeUeConn(t.Context(), ue)

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the UE returned to ECM-IDLE, want Idle", state)
	}

	if pending := ue.PagingPending(); pending != nil {
		t.Errorf("pending = %+v, want the undelivered transfer released", pending)
	}
}

// TS 23.401 §5.3.5: an Initial Context Setup awaiting its response is pending MT
// signalling, so a user-inactivity release is held until it settles.
func TestOutstandingInitialContextSetupIsPendingMTSignalling(t *testing.T) {
	m := newTestMME(t)
	conn := m.NewUeConn(&captureConn{}, 9)

	if conn.MTSignallingPending() {
		t.Fatal("a connection with nothing in flight reports pending MT signalling")
	}

	conn.SetICS(ICSPending)

	if !conn.MTSignallingPending() {
		t.Error("an Initial Context Setup awaiting its response does not count as pending MT signalling")
	}

	conn.SetICS(ICSCompleted)

	if conn.MTSignallingPending() {
		t.Error("a completed Initial Context Setup still counts as pending MT signalling")
	}
}

func TestStalePagingAbortSparesANewerAttempt(t *testing.T) {
	ue := NewUeContext()
	newer := &MTRequest{}

	ue.paging.mu.Lock()
	ue.paging.attempt = 2
	ue.paging.pending = newer
	ue.paging.state = PagingAttempting
	ue.paging.mu.Unlock()

	if _, abandoned := ue.PagingUnanswered(t.Context(), 1, models.EPSPagingUENotResponding); abandoned {
		t.Fatal("the abort of paging attempt 1 abandoned attempt 2")
	}

	if ue.PagingPending() != newer {
		t.Fatal("the abort of paging attempt 1 dropped the request attempt 2 is delivering")
	}
}

func TestPagingDoesNotBeginForAUEThatHasReconnected(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	if ue.beginPaging(&MTRequest{Ebi: 5}) {
		t.Fatal("beginPaging on a connected UE began paging")
	}

	if ue.PagingState() != PagingIdle {
		t.Fatalf("paging state = %s after the UE reconnected, want Idle: nothing would ever deliver the buffered request", ue.PagingState())
	}
}
