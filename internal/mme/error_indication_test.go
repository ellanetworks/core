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
