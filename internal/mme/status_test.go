// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
)

func TestConnectedSubscribers(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	registered := m.NewUe(conn, 7)
	registerTestUE(m, registered, "001010000000001")
	registered.ForceStateForTest(EMMRegistered)
	registered.cipheringAlg = 2
	registered.integrityAlg = 2
	registered.Imei, _ = etsi.NewIMEIFromPEI("353456789012347")
	testPDN(registered).Apn = "internet"
	registered.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("5 Gbps"), Downlink: models.MustParseBitRate("6 Gbps")}
	testPDN(registered).SessAmbrULBps = models.MustParseBitRate("1 Gbps").Bps()
	testPDN(registered).SessAmbrDLBps = models.MustParseBitRate("2 Gbps").Bps()
	testPDN(registered).UeIP = netip.MustParseAddr("10.45.0.2")
	registered.TouchLastSeen()

	deregistered := m.NewUe(conn, 8)
	registerTestUE(m, deregistered, "001010000000002")
	deregistered.ForceStateForTest(EMMDeregistered)

	noIMSI := m.NewUe(conn, 9)
	noIMSI.ForceStateForTest(EMMRegistered)

	got := m.ConnectedSubscribers()

	if len(got) != 1 {
		t.Fatalf("ConnectedSubscribers returned %d entries, want 1: %+v", len(got), got)
	}

	st, ok := got["001010000000001"]
	if !ok {
		t.Fatalf("registered subscriber missing from %+v", got)
	}

	if st.NumSessions != 1 {
		t.Fatalf("NumSessions = %d, want 1", st.NumSessions)
	}

	if st.CipheringAlgorithm != "EEA2" || st.IntegrityAlgorithm != "EIA2" {
		t.Fatalf("algorithms = %q/%q, want EEA2/EIA2", st.CipheringAlgorithm, st.IntegrityAlgorithm)
	}

	if len(st.Sessions) != 1 {
		t.Fatalf("Sessions = %d, want 1 default bearer", len(st.Sessions))
	}

	session := st.Sessions[0]
	if session.APN != "internet" || session.IPv4Address != "10.45.0.2" || session.BearerID != DefaultERABID {
		t.Fatalf("session = %+v, want APN internet / IP 10.45.0.2 / bearer %d", session, DefaultERABID)
	}

	if st.Imei != "353456789012347" {
		t.Fatalf("Imei = %q, want 353456789012347", st.Imei)
	}

	if st.LastSeenAt.IsZero() {
		t.Fatal("LastSeenAt is zero, want the touched time")
	}

	if session.AMBRUplink != "1 Gbps" || session.AMBRDownlink != "2 Gbps" {
		t.Fatalf("session AMBR = %q/%q, want 1 Gbps/2 Gbps", session.AMBRUplink, session.AMBRDownlink)
	}
}

func TestStatusIncludesIdleSubscriber(t *testing.T) {
	m := newTestMME(t)

	ue, _ := securedUE(t, m)
	registerTestUE(m, ue, "001010000000001")
	ue.ForceStateForTest(EMMRegistered)
	testPDN(ue).Apn = "internet"

	m.FreeUeConn(ue)

	if ue.Connected() {
		t.Fatal("UE still connected after FreeUeConn")
	}

	if got := m.CountRegisteredSubscribers(); got != 1 {
		t.Fatalf("idle registered subscriber not counted: got %d", got)
	}

	if _, ok := m.LookupSubscriber("001010000000001"); !ok {
		t.Fatal("idle registered subscriber not found by LookupSubscriber")
	}

	if _, ok := m.ConnectedSubscribers()["001010000000001"]; !ok {
		t.Fatal("idle registered subscriber missing from ConnectedSubscribers")
	}

	m.RemoveUe(ue)
}

func TestMobileIdentityDigitsIMEISV(t *testing.T) {
	imeisv := []byte{0x03, 0x53, 0x60, 0x83, 0x12, 0x34, 0x56, 0x78, 0xf0}

	got := mobileIdentityDigits(imeisv)
	if got != "0350638214365870" {
		t.Fatalf("mobileIdentityDigits = %q, want 0350638214365870", got)
	}
}

func TestLookupSubscriber(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	ue := m.NewUe(conn, 7)
	registerTestUE(m, ue, "001010000000001")
	ue.ForceStateForTest(EMMRegistered)

	if _, ok := m.LookupSubscriber("001010000000099"); ok {
		t.Fatal("LookupSubscriber found an unknown IMSI")
	}

	if _, ok := m.LookupSubscriber("001010000000001"); !ok {
		t.Fatal("LookupSubscriber did not find the registered IMSI")
	}
}

func TestCountRegisteredSubscribers(t *testing.T) {
	m := newTestMME(t)
	conn := new(sctp.SCTPConn)

	a := m.NewUe(conn, 7)
	registerTestUE(m, a, "001010000000001")
	a.ForceStateForTest(EMMRegistered)

	b := m.NewUe(conn, 8)
	registerTestUE(m, b, "001010000000002")
	b.ForceStateForTest(EMMDeregistered)

	if got := m.CountRegisteredSubscribers(); got != 1 {
		t.Fatalf("CountRegisteredSubscribers = %d, want 1", got)
	}
}

func TestHasENBAndCount(t *testing.T) {
	m := newTestMME(t)
	m.trackRadio(new(sctp.SCTPConn), RadioInfo{Name: "enb-a", ID: "00f110-1"})
	m.trackRadio(new(sctp.SCTPConn), RadioInfo{Name: "enb-b", ID: "00f110-2"})

	if !m.HasRadio("enb-a") {
		t.Fatal("HasRadio(enb-a) = false, want true")
	}

	if m.HasRadio("enb-z") {
		t.Fatal("HasRadio(enb-z) = true, want false")
	}

	if got := m.CountRadios(); got != 2 {
		t.Fatalf("CountRadios = %d, want 2", got)
	}
}

func TestLastSeenRadioSurvivesIdleAndDeregistration(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	ue := m.NewUe(conn, 7)
	registerTestUE(m, ue, "001010000000001")
	ue.ForceStateForTest(EMMRegistered)

	if err := m.CommitUEIdentity(t.Context(), ue, MintAuthProofForAttachCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	cs, ok := m.LookupSubscriber("001010000000001")
	if !ok || !cs.Connected {
		t.Fatalf("connected UE: found=%v Connected=%v, want true/true", ok, cs.Connected)
	}

	m.FreeUeConn(ue)

	cs, ok = m.LookupSubscriber("001010000000001")
	if !ok {
		t.Fatal("idle UE missing from LookupSubscriber")
	}

	if cs.Connected {
		t.Error("Connected = true for a UE with no S1-connection")
	}

	if seen, ok := m.LastSeen("001010000000001"); !ok || seen.RadioName != "enb-a" {
		t.Errorf("idle UE last-seen radio = %q (found %v), want enb-a", seen.RadioName, ok)
	}

	m.RemoveUe(ue)

	if _, ok := m.LookupSubscriber("001010000000001"); ok {
		t.Fatal("deregistered UE still reported as registered")
	}

	if seen, ok := m.LastSeen("001010000000001"); !ok || seen.RadioName != "enb-a" {
		t.Errorf("deregistered UE last-seen radio = %q (found %v), want it retained as enb-a", seen.RadioName, ok)
	}

	m.ForgetSubscriber("001010000000001")

	if _, ok := m.LastSeen("001010000000001"); ok {
		t.Error("ForgetSubscriber left the retained record in place")
	}
}

func TestLastSeenRadioFollowsARename(t *testing.T) {
	const imsi = "001010000000001"

	m := newTestMME(t)
	conn := connectENB(t, m, "enb-a", 1)

	ue := m.NewUe(conn, 7)
	registerTestUE(m, ue, imsi)
	ue.ForceStateForTest(EMMRegistered)

	if err := m.CommitUEIdentity(t.Context(), ue, MintAuthProofForAttachCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	m.FreeUeConn(ue)

	m.UpdateRadioName(m.RadioForConn(conn), "enb-a-renamed")

	seen, ok := m.LastSeen(imsi)
	if !ok {
		t.Fatal("retained record missing after rename")
	}

	if seen.RadioName != "enb-a-renamed" {
		t.Errorf("RadioName = %q, want the current name enb-a-renamed", seen.RadioName)
	}
}

func TestLastSeenRadioFallsBackToTheCapturedName(t *testing.T) {
	const imsi = "001010000000001"

	m := newTestMME(t)
	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-unclaimed"})

	ue := m.NewUe(conn, 7)
	registerTestUE(m, ue, imsi)
	ue.ForceStateForTest(EMMRegistered)

	if err := m.CommitUEIdentity(t.Context(), ue, MintAuthProofForAttachCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	seen, ok := m.LastSeen(imsi)
	if !ok {
		t.Fatal("retained record missing")
	}

	if seen.RadioID != "" {
		t.Errorf("RadioID = %q, want empty for a radio with no Global eNB ID", seen.RadioID)
	}

	if seen.RadioName != "enb-unclaimed" {
		t.Errorf("RadioName = %q, want the captured name enb-unclaimed", seen.RadioName)
	}
}

func TestLastSeenRadioFollowsAnX2PathSwitch(t *testing.T) {
	const imsi = "001010000000001"

	m := newTestMME(t)
	source := connectENB(t, m, "enb-a", 1)
	target := connectENB(t, m, "enb-b", 2)

	ue := m.NewUe(source, 7)
	registerTestUE(m, ue, imsi)
	ue.ForceStateForTest(EMMRegistered)

	if err := m.CommitUEIdentity(t.Context(), ue, MintAuthProofForAttachCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.TouchLastSeen()

	if _, ok := m.CommitPathSwitch(ue, target, 9, [32]byte{}, 0); !ok {
		t.Fatal("CommitPathSwitch reported the UE released")
	}

	m.FreeUeConn(ue)

	seen, ok := m.LastSeen(imsi)
	if !ok {
		t.Fatal("retained record missing after the path switch")
	}

	if seen.RadioName != "enb-b" {
		t.Errorf("RadioName = %q, want the target enb-b", seen.RadioName)
	}
}
