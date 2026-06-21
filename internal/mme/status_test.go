// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/sctp"
)

func TestConnectedSubscribers(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackENB(conn, ENBInfo{Name: "enb-a", ID: "00f110-1"})

	registered := m.newUe(conn, 7)
	registered.imsi = "001010000000001"
	registered.emmState = EMMRegistered
	registered.eea = 2
	registered.eia = 2
	registered.imei = "353456789012347"
	testPDN(registered).apn = "internet"
	registered.ambrUplink = "1 Gbps"
	registered.ambrDownlink = "2 Gbps"
	testPDN(registered).ueIP = netip.MustParseAddr("10.45.0.2")
	registered.touchLastSeen()

	deregistered := m.newUe(conn, 8)
	deregistered.imsi = "001010000000002"
	deregistered.emmState = EMMDeregistered

	noIMSI := m.newUe(conn, 9)
	noIMSI.emmState = EMMRegistered

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
	if session.APN != "internet" || session.IPv4Address != "10.45.0.2" || session.BearerID != defaultERABID {
		t.Fatalf("session = %+v, want APN internet / IP 10.45.0.2 / bearer %d", session, defaultERABID)
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
	m.trackENB(conn, ENBInfo{Name: "enb-a", ID: "00f110-1"})

	ue := m.newUe(conn, 7)
	ue.imsi = "001010000000001"
	ue.emmState = EMMRegistered

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

	a := m.newUe(conn, 7)
	a.imsi = "001010000000001"
	a.emmState = EMMRegistered

	b := m.newUe(conn, 8)
	b.imsi = "001010000000002"
	b.emmState = EMMDeregistered

	if got := m.CountRegisteredSubscribers(); got != 1 {
		t.Fatalf("CountRegisteredSubscribers = %d, want 1", got)
	}
}

func TestHasENBAndCount(t *testing.T) {
	m := newTestMME(t)
	m.trackENB(new(sctp.SCTPConn), ENBInfo{Name: "enb-a", ID: "00f110-1"})
	m.trackENB(new(sctp.SCTPConn), ENBInfo{Name: "enb-b", ID: "00f110-2"})

	if !m.HasENB("enb-a") {
		t.Fatal("HasENB(enb-a) = false, want true")
	}

	if m.HasENB("enb-z") {
		t.Fatal("HasENB(enb-z) = true, want false")
	}

	if got := m.CountENBs(); got != 2 {
		t.Fatalf("CountENBs = %d, want 2", got)
	}
}
