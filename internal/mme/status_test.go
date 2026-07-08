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
	registered.Ambr = &models.Ambr{Uplink: "1 Gbps", Downlink: "2 Gbps"}
	testPDN(registered).UeIP = netip.MustParseAddr("10.45.0.2")
	registered.TouchLastSeen()

	deregistered := m.NewUe(conn, 8)
	registerTestUE(m, deregistered, "001010000000002")
	deregistered.ForceStateForTest(EMMDeregistered)

	// A registered context with no IMSI is never indexed by subscriber identity,
	// so it is excluded from the status surface.
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

	if st.RadioName != "enb-a" {
		t.Fatalf("RadioName = %q, want %q", st.RadioName, "enb-a")
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

// TestStatusIncludesIdleSubscriber confirms a registered UE that has moved to
// ECM-IDLE (no S1 connection) is still reported by the status surface, with no
// radio name.
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

	cs, ok := m.LookupSubscriber("001010000000001")
	if !ok {
		t.Fatal("idle registered subscriber not found by LookupSubscriber")
	}

	if cs.RadioName != "" {
		t.Fatalf("idle subscriber RadioName = %q, want empty", cs.RadioName)
	}

	if _, ok := m.ConnectedSubscribers()["001010000000001"]; !ok {
		t.Fatal("idle registered subscriber missing from ConnectedSubscribers")
	}

	m.RemoveUe(ue) // stop the default-duration timer
}

func TestMobileIdentityDigitsIMEISV(t *testing.T) {
	// IMEISV mobile identity (TS 24.008 §10.5.1.4): octet 0 carries the type and
	// the first digit; the rest is packed BCD with a trailing 0xF filler.
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

	cs, ok := m.LookupSubscriber("001010000000001")
	if !ok {
		t.Fatal("LookupSubscriber did not find the registered IMSI")
	}

	if cs.RadioName != "enb-a" {
		t.Fatalf("RadioName = %q, want enb-a", cs.RadioName)
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
