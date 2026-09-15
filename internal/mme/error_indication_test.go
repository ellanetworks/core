// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func TestErrorIndicationReleasesS1OnAConnectedUE(t *testing.T) {
	m := newTestMME(t)

	ue := idleRegisteredUE(t, m)
	conn := &captureConn{}
	establishResumeForTest(m, ue, conn, 9)

	if !ue.Connected() {
		t.Fatal("the UE should be ECM-CONNECTED for this case")
	}

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataErrorIndication); err != nil {
		t.Fatalf("NotifyDownlinkData: %v", err)
	}

	conn.mu.Lock()
	sent := len(conn.sent)
	conn.mu.Unlock()

	if sent == 0 {
		t.Fatal("no UE Context Release Command was sent to the eNB")
	}

	if !m.Session.(*fakeSessionManager).deactivated {
		t.Error("the access bearers were not released before the UE Context Release Command (TS 23.401 §5.3.5)")
	}

	if m.pagingActive(ue) {
		t.Error("the UE was paged while it was still ECM-CONNECTED")
	}

	if ue.PagingPending() != nil {
		t.Error("a paging procedure was started before S1 was released")
	}
}

func TestErrorIndicationPagesAConnectedUEOnceS1IsReleased(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)
	establishResumeForTest(m, ue, &captureConn{}, 9)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataErrorIndication); err != nil {
		t.Fatalf("NotifyDownlinkData: %v", err)
	}

	m.ReleaseUEContextLocally(t.Context(), ue, "test-release-complete")

	if ue.Connected() {
		t.Fatal("the UE should be ECM-IDLE once the release completed")
	}

	if !m.pagingActive(ue) {
		t.Fatal("the UE was never paged after S1 was released for a GTP-U Error Indication (TS 23.007 clause 22)")
	}

	pending := ue.PagingPending()
	if pending == nil || pending.Ebi != 5 {
		t.Errorf("paging pending = %+v, want the EBI the anchor reported", pending)
	}
}

func TestErrorIndicationDropsTheServiceRequestWhenTheUEIsNotRegistered(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)
	establishResumeForTest(m, ue, &captureConn{}, 9)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataErrorIndication); err != nil {
		t.Fatalf("NotifyDownlinkData: %v", err)
	}

	ue.ForceStateForTest(EMMDeregistered)
	m.ReleaseUEContextLocally(t.Context(), ue, "test-release-complete")

	if m.pagingActive(ue) {
		t.Error("a deregistered UE was paged")
	}

	if ue.takeDeferredServiceRequest() != nil {
		t.Error("the deferred service request was not abandoned with the UE context")
	}

	if got := m.Session.(*fakeSessionManager).suppressCalls; got == 0 {
		t.Error("the anchor was never told to stop buffering for a UE that will not be paged")
	}
}

func TestDownlinkDataArrivalDoesNotReleaseS1(t *testing.T) {
	m := newTestMME(t)

	ue := idleRegisteredUE(t, m)
	conn := &captureConn{}
	establishResumeForTest(m, ue, conn, 11)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("NotifyDownlinkData: %v", err)
	}

	conn.mu.Lock()
	sent := len(conn.sent)
	conn.mu.Unlock()

	if sent != 0 {
		t.Errorf("downlink data arrival released S1: %d messages sent to the eNB", sent)
	}
}

func TestErrorIndicationPagesAnIdleUE(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataErrorIndication); err != nil {
		t.Fatalf("NotifyDownlinkData: %v", err)
	}

	if !m.pagingActive(ue) {
		t.Error("an idle UE was not paged after an Error Indication")
	}
}
