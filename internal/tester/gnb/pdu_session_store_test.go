// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"sync"
	"testing"
	"time"
)

func TestPDUSessionsForReportsTheRANSideView(t *testing.T) {
	g := &GnodeB{}
	g.cond = sync.NewCond(&g.mu)

	g.storePDUSession(7, &PDUSessionInformation{PDUSessionID: 5})
	g.storePDUSession(7, &PDUSessionInformation{PDUSessionID: 1})
	g.storePDUSession(9, &PDUSessionInformation{PDUSessionID: 3})

	got := g.pduSessionsFor(7)
	if len(got) != 2 || got[0].PDUSessionID != 1 || got[1].PDUSessionID != 5 {
		t.Fatalf("pduSessionsFor(7) = %+v, want PDU sessions 1 and 5", got)
	}

	if other := g.pduSessionsFor(9); len(other) != 1 {
		t.Errorf("pduSessionsFor(9) = %+v, want one session; the store must be per UE", other)
	}
}

func TestDropPDUSessionsClearsTheUEAfterAContextRelease(t *testing.T) {
	g := &GnodeB{}
	g.cond = sync.NewCond(&g.mu)

	g.storePDUSession(7, &PDUSessionInformation{PDUSessionID: 1})
	g.storePDUSession(9, &PDUSessionInformation{PDUSessionID: 3})

	g.dropPDUSessions(7)

	if got := g.pduSessionsFor(7); len(got) != 0 {
		t.Errorf("pduSessionsFor(7) = %+v after the UE context was released, want none: the NG-RAN node holds no N3 user plane for it", got)
	}

	if got := g.pduSessionsFor(9); len(got) != 1 {
		t.Errorf("pduSessionsFor(9) = %+v, want one session; releasing one UE must not affect another", got)
	}
}

func TestRadioCapabilityReportIsClaimedOncePerUEAndPrunedOnRelease(t *testing.T) {
	g := &GnodeB{UERadioCapability: DefaultUERadioCapability}
	g.cond = sync.NewCond(&g.mu)

	if !g.claimRadioCapabilityReport(7) {
		t.Fatal("the first report for a UE must be claimable")
	}

	if g.claimRadioCapabilityReport(7) {
		t.Error("a live gNB reports the capability once per UE context setup, not on every one")
	}

	g.dropRadioCapabilityReport(7)

	if !g.claimRadioCapabilityReport(7) {
		t.Error("a released UE context must not keep its report claimed; the entry leaks otherwise")
	}
}

func TestRadioCapabilityReportDisabledWhenEmpty(t *testing.T) {
	g := &GnodeB{}
	g.cond = sync.NewCond(&g.mu)

	if g.claimRadioCapabilityReport(7) {
		t.Error("an empty capability must disable the report")
	}
}

func TestAwaitPDUSessionReleaseReturnsWhenTheSessionIsDropped(t *testing.T) {
	g := &GnodeB{}
	g.cond = sync.NewCond(&g.mu)

	g.storePDUSession(7, &PDUSessionInformation{PDUSessionID: 1})

	go func() {
		time.Sleep(10 * time.Millisecond)
		g.removePDUSession(7, 1)
	}()

	if err := g.awaitPDUSessionRelease(7, 1, time.Second); err != nil {
		t.Fatalf("awaitPDUSessionRelease: %v", err)
	}
}

func TestAwaitPDUSessionReleaseTimesOutWhileTheSessionStands(t *testing.T) {
	g := &GnodeB{}
	g.cond = sync.NewCond(&g.mu)

	g.storePDUSession(7, &PDUSessionInformation{PDUSessionID: 1})

	if err := g.awaitPDUSessionRelease(7, 1, 50*time.Millisecond); err == nil {
		t.Error("expected a timeout while the gNB still holds the PDU session")
	}
}
