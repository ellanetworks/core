// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
)

func testENBID(value uint32) s1ap.GlobalENBID {
	return s1ap.GlobalENBID{
		PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
		ENBID:        s1ap.ENBID{Value: value},
	}
}

func testRanNodeID(t *testing.T, value uint32) models.GlobalRanNodeID {
	t.Helper()

	ranID, err := RanNodeID(testENBID(value))
	if err != nil {
		t.Fatalf("test eNB %d has no RAN node ID: %v", value, err)
	}

	return ranID
}

func mustRanNodeID(t *testing.T, g s1ap.GlobalENBID) models.GlobalRanNodeID {
	t.Helper()

	ranID, err := RanNodeID(g)
	if err != nil {
		t.Fatalf("RanNodeID: %v", err)
	}

	return ranID
}

func claimENBID(t *testing.T, m *MME, radio *Radio, g s1ap.GlobalENBID) {
	t.Helper()

	if err := m.ClaimENBID(t.Context(), radio, g, DefaultRelativeCapacity); err != nil {
		t.Fatalf("ClaimENBID: %v", err)
	}
}

func testENBKey(t *testing.T, value uint32) string {
	t.Helper()

	key, ok := testRanNodeID(t, value).Ref()
	if !ok {
		t.Fatalf("test eNB %d has no registry key", value)
	}

	return key
}

func TestENBTable(t *testing.T) {
	m := newTestMME(t)

	c1 := new(sctp.SCTPConn)
	c2 := new(sctp.SCTPConn)

	m.trackRadio(context.Background(), c1, RadioInfo{Name: "enb-a", ID: "00f110-1"})
	m.trackRadio(context.Background(), c2, RadioInfo{Name: "enb-b", ID: "00f110-2"})

	if got := len(m.ListRadios()); got != 2 {
		t.Fatalf("ListRadios = %d, want 2", got)
	}

	m.DisconnectRadio(c1)

	got := m.ListRadios()
	if len(got) != 1 || got[0].Name != "enb-b" {
		t.Fatalf("after remove: %+v", got)
	}

	m.DisconnectRadio(new(sctp.SCTPConn))

	if got := len(m.ListRadios()); got != 1 {
		t.Fatalf("ListRadios = %d, want 1", got)
	}
}

// TS 36.413 §8.7.3.1
func TestENBSetupCompleteGate(t *testing.T) {
	m := newTestMME(t)
	c := new(sctp.SCTPConn)

	setupComplete := func(conn *sctp.SCTPConn) bool {
		r := m.RadioForConn(conn)
		return r != nil && r.SetupComplete()
	}

	if setupComplete(c) {
		t.Fatal("untracked eNB reported setup-complete")
	}

	m.trackRadio(context.Background(), c, RadioInfo{Name: "enb-a"})

	if setupComplete(c) {
		t.Fatal("tracked-but-not-set-up eNB reported setup-complete")
	}

	claimENBID(t, m, m.RadioForConn(c), testENBID(1))

	if !setupComplete(c) {
		t.Fatal("eNB not setup-complete after claiming its Global eNB ID")
	}

	m.DisconnectRadio(c)

	if setupComplete(c) {
		t.Fatal("removed eNB still setup-complete")
	}
}

func TestClaimENBID_EvictsStaleReassociation(t *testing.T) {
	m := newTestMME(t)

	enbID := testENBID(1)
	id := testENBKey(t, 1)

	c1 := new(sctp.SCTPConn)
	c2 := new(sctp.SCTPConn)

	m.trackRadio(context.Background(), c1, RadioInfo{Name: "enb-old"})
	claimENBID(t, m, m.RadioForConn(c1), enbID)

	if got, ok := m.reg.ClaimedBy(id); !ok || got.Conn != S1APWriter(c1) {
		t.Fatalf("setup: the Global eNB ID %q resolved to the wrong association", id)
	}

	m.trackRadio(context.Background(), c2, RadioInfo{Name: "enb-new"})
	claimENBID(t, m, m.RadioForConn(c2), enbID)

	if got, ok := m.reg.ClaimedBy(id); !ok || got.Conn != S1APWriter(c2) {
		t.Errorf("the Global eNB ID %q did not resolve to the re-associated eNB", id)
	}

	if r := m.RadioForConn(c1); r != nil && r.SetupComplete() {
		t.Error("stale association should have been evicted from the eNB table")
	}

	if r := m.RadioForConn(c2); r == nil || !r.SetupComplete() {
		t.Error("current association should be setup-complete")
	}

	if got := m.ListRadios(); len(got) != 1 || got[0].Name != "enb-new" {
		t.Errorf("ListRadios = %+v, want only enb-new", got)
	}
}

// TS 36.413 §8.7.3.1
func TestClaimENBID_RepeatOnSameAssociationReleasesUEs(t *testing.T) {
	m := newTestMME(t)

	enbID := testENBID(1)
	c := new(sctp.SCTPConn)

	m.trackRadio(context.Background(), c, RadioInfo{Name: "enb-a"})
	claimENBID(t, m, m.RadioForConn(c), enbID)

	m.NewUeConn(c, 10)

	if got := len(m.ConnsOnConn(c)); got != 1 {
		t.Fatalf("setup: expected 1 UE connection, got %d", got)
	}

	claimENBID(t, m, m.RadioForConn(c), enbID)

	if got := len(m.ConnsOnConn(c)); got != 0 {
		t.Fatalf("expected the eNB's UE contexts to be released, %d remain", got)
	}
}

func TestClaimENBID_KindIsPartOfTheIdentity(t *testing.T) {
	m := newTestMME(t)

	macro := testENBID(8)

	home := testENBID(8)
	home.ENBID.Kind = s1ap.ENBIDHome

	c1 := new(sctp.SCTPConn)
	c2 := new(sctp.SCTPConn)

	m.trackRadio(context.Background(), c1, RadioInfo{Name: "enb-macro"})
	claimENBID(t, m, m.RadioForConn(c1), macro)

	m.trackRadio(context.Background(), c2, RadioInfo{Name: "enb-home"})
	claimENBID(t, m, m.RadioForConn(c2), home)

	first, ok := m.FindConnectedRadioByRanID(mustRanNodeID(t, macro))
	if !ok || first.Conn != S1APWriter(c1) {
		t.Error("the macro eNB no longer resolves to its own association")
	}

	second, ok := m.FindConnectedRadioByRanID(mustRanNodeID(t, home))
	if !ok || second.Conn != S1APWriter(c2) {
		t.Error("the home eNB does not resolve to its own association")
	}

	if got := len(m.ListRadios()); got != 2 {
		t.Errorf("ListRadios() = %d eNBs, want 2", got)
	}
}

func TestRepeatS1SetupReusesTheAssociationsRadio(t *testing.T) {
	m := newTestMME(t)

	c := new(sctp.SCTPConn)
	id := testENBKey(t, 1)

	m.trackRadio(context.Background(), c, RadioInfo{Name: "enb-a"})

	first := m.RadioForConn(c)
	if first == nil {
		t.Fatal("S1 Setup did not track the eNB")
	}

	claimENBID(t, m, first, testENBID(1))

	m.trackRadio(context.Background(), c, RadioInfo{Name: "enb-a-renamed"})

	again := m.RadioForConn(c)
	if again != first {
		t.Fatal("a repeat S1 Setup replaced the association's Radio instead of re-surveying it")
	}

	if again.NodeName() != "enb-a" {
		t.Errorf("an unanswered repeat S1 Setup changed the eNB name, got %q", again.NodeName())
	}

	if !again.SetupComplete() {
		t.Error("an unanswered repeat S1 Setup took the eNB out of setup-complete; only an accepted one replaces the setup (TS 36.413 §8.7.3.3)")
	}

	if _, ok := m.reg.ClaimedBy(id); !ok {
		t.Errorf("the Global eNB ID %q was unclaimed by an unanswered repeat S1 Setup", id)
	}

	if len(m.ListRadios()) != 1 {
		t.Errorf("a repeat S1 Setup changed the connected eNB count, got %d", len(m.ListRadios()))
	}
}
