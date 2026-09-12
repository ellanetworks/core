// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestPagingFailedReportsTheCauseForThePendingBearer(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5, nil); err != nil {
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

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5, nil); err != nil {
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

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5, nil); err != nil {
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

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5, nil); err != nil {
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

	if err := m.Page(context.Background(), ue.imsiOrEmpty(), 5, nil); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingAnswered()

	m.ReleaseUEContextLocally(ue, "test")

	if state := ue.PagingState(); state != PagingIdle {
		t.Errorf("paging state = %s after the connection carrying the delivery was released, want Idle", state)
	}
}
