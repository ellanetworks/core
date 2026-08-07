// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"net/netip"
	"testing"
)

func testEngine(t *testing.T, n3v6 string) *SessionEngine {
	t.Helper()

	const n3v4 = "10.3.0.2"

	conn, err := NewSessionEngine("1.2.3.4", "nodeId", n3v4, n3v6, n3v4, n3v6, nil, nil)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	return conn
}

func sessionHolding(pdrs ...SPDRInfo) *Session {
	sess := NewSession(1)
	for _, pdr := range pdrs {
		sess.PutPDR(pdr.PdrID, pdr)
	}

	return sess
}

func TestAppliedStateReportsUplinkFTEID(t *testing.T) {
	conn := testEngine(t, "")
	n3Addr := netip.MustParseAddr("10.3.0.2")

	applied := conn.appliedState(sessionHolding(SPDRInfo{PdrID: 1, TeID: 42}))

	if len(applied.UplinkPDRs) != 1 {
		t.Fatalf("uplink PDRs = %d, want 1", len(applied.UplinkPDRs))
	}

	got := applied.UplinkPDRs[0]
	if got.PDRID != 1 || got.TEID != 42 || got.N3IPv4 != n3Addr {
		t.Errorf("applied PDR = %+v, want PDRID 1, TEID 42, N3IPv4 %v", got, n3Addr)
	}
}

func TestAppliedStateExcludesDownlinkPDR(t *testing.T) {
	conn := testEngine(t, "")

	applied := conn.appliedState(sessionHolding(SPDRInfo{PdrID: 2, UEIP: netip.MustParseAddr("10.0.0.1")}))

	if len(applied.UplinkPDRs) != 0 {
		t.Fatalf("uplink PDRs = %+v, want none for a downlink-only session", applied.UplinkPDRs)
	}
}

// The applied state is the UPF's whole answer, not a record of what the last
// apply changed: a session's local F-TEID is reported on every apply, whether
// or not that apply is the one that allocated it.
func TestAppliedStateReportsHeldFTEIDNotJustNewlyAllocated(t *testing.T) {
	conn := testEngine(t, "")

	applied := conn.appliedState(sessionHolding(SPDRInfo{PdrID: 1, TeID: 42, Allocated: false}))

	if len(applied.UplinkPDRs) != 1 || applied.UplinkPDRs[0].TEID != 42 {
		t.Fatalf("uplink PDRs = %+v, want TEID 42 restated", applied.UplinkPDRs)
	}
}

func TestAppliedStateCarriesBothN3Families(t *testing.T) {
	conn := testEngine(t, "2001:db8::1")

	n3v4 := netip.MustParseAddr("10.3.0.2")
	n3v6 := netip.MustParseAddr("2001:db8::1")

	applied := conn.appliedState(sessionHolding(SPDRInfo{PdrID: 1, TeID: 42}))

	if len(applied.UplinkPDRs) != 1 {
		t.Fatalf("uplink PDRs = %d, want 1", len(applied.UplinkPDRs))
	}

	if applied.UplinkPDRs[0].N3IPv4 != n3v4 || applied.UplinkPDRs[0].N3IPv6 != n3v6 {
		t.Errorf("applied N3 = %v / %v, want %v / %v",
			applied.UplinkPDRs[0].N3IPv4, applied.UplinkPDRs[0].N3IPv6, n3v4, n3v6)
	}
}
