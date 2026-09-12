// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func TestPagingFailedReportsTheCauseForThePendingBearer(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5); err != nil {
		t.Fatalf("Page: %v", err)
	}

	if state := ue.PagingState(); state != PagingAttempting {
		t.Fatalf("paging state = %s after Page, want Attempting", state)
	}

	dropped := ue.PagingFailed(models.EPSPagingUENotResponding)
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

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingAnswered()

	if state := ue.PagingState(); state != PagingDelivering {
		t.Errorf("paging state = %s after the UE answered, want Delivering", state)
	}

	ue.PagingDelivered()

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after delivery, want Idle", state)
	}
}

func TestClearPagingDropsTheBufferedLPPa(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	ue.SetLPPaBuffered(7, []byte{0x01})

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingFailed(models.EPSPagingUENotResponding)

	if ue.PopLPPaBuffered() != nil {
		t.Error("the buffered LPPa payload survived the failed paging procedure")
	}
}

func TestDetachFailsThePendingTransfer(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.TransitionTo(EMMDeregistered)

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the UE was deregistered, want Idle", state)
	}
}

func TestConnectionReleaseFailsADeliveringTransfer(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingAnswered()

	m.ReleaseUEContextLocally(ue, "test")

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

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.SetLPPaBuffered(7, []byte{0x01})

	ue.Pdns = map[uint8]*PdnConnection{
		5: {Ebi: 5},
		6: {Ebi: 6},
	}

	m.AttachUeConn(ue, m.NewUeConn(&captureConn{}, 9))

	m.abandonPaging(ue)

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

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5); err != nil {
		t.Fatalf("Page: %v", err)
	}

	m.AttachUeConn(ue, m.NewUeConn(&captureConn{}, 9))

	if state := ue.PagingState(); state != PagingDelivering {
		t.Fatalf("paging state = %s after the UE answered, want Delivering", state)
	}

	m.FreeUeConn(ue)

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
