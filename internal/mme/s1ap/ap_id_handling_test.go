// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func releaseCompleteValue(t *testing.T, mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) []byte {
	t.Helper()

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(mmeID), ENBUES1APID: s1ap.Ptr(enbID)}

	return successfulValue(t, mustMarshal(t, complete.Marshal))
}

func requireIdleAndRetained(t *testing.T, m *mme.MME, ue *mme.UeContext) {
	t.Helper()

	if ue.Connected() {
		t.Fatal("the S1 connection holding the erroneous S1AP ID was not released locally")
	}

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); !ok {
		t.Fatal("the local release of an S1 connection deleted the registered UE context")
	}
}

// TS 36.413 §10.6
func TestNonLastMessageWithUnknownMMEUES1APIDReleasesTheConnectionHoldingItsENBUES1APID(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)

	if _, _, ok := resolveUE(t.Context(), m, conn, 4242, ue.Conn().ENBUES1APID()); ok {
		t.Fatal("an unknown MME-UE-S1AP-ID resolved")
	}

	if conn.count() != 1 {
		t.Fatalf("sent %d messages, want only the Error Indication", conn.count())
	}

	parseOutboundErrorIndication(t, conn.sent[0])
	requireIdleAndRetained(t, m, ue)
}

// TS 36.413 §10.6
func TestNonLastMessageWithInconsistentPairReleasesTheConnectionLocally(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)

	if _, _, ok := resolveUE(t.Context(), m, conn, ue.Conn().MMEUES1APID, 99); ok {
		t.Fatal("an inconsistent pair resolved")
	}

	if conn.count() != 1 {
		t.Fatalf("sent %d messages, want only the Error Indication", conn.count())
	}

	parseOutboundErrorIndication(t, conn.sent[0])
	requireIdleAndRetained(t, m, ue)
}

// TS 36.413 §10.6
func TestNonLastMessageFromAnotherENBReleasesNothingOnTheUEsInterface(t *testing.T) {
	m := newTestMME(t)
	ue, home := securedUE(t, m)
	foreign := &captureConn{}

	if _, _, ok := resolveUE(t.Context(), m, foreign, ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID()); ok {
		t.Fatal("a pair resolved on another S1 interface")
	}

	if foreign.count() != 1 {
		t.Fatalf("sent %d messages to the sending eNB, want the Error Indication", foreign.count())
	}

	if home.count() != 0 || !ue.Connected() || ue.Conn().ConnForTest() != home {
		t.Fatal("a message from another eNB released the UE's S1 connection")
	}
}

// TS 36.413 §10.6
func TestReleaseCompleteWithUnknownMMEUES1APIDLeavesTheConnectionHoldingItsENBUES1APID(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)

	HandleUEContextReleaseComplete(t.Context(), m, mme.NewRadioForTest(conn), releaseCompleteValue(t, 4242, ue.Conn().ENBUES1APID()))

	if conn.count() != 0 {
		t.Fatalf("sent %d messages in answer to the last message, want none", conn.count())
	}

	if !ue.Connected() || ue.Conn().ConnForTest() != conn {
		t.Fatal("a Release Complete naming an MME-UE-S1AP-ID the MME does not hold released the connection holding its eNB-UE-S1AP-ID")
	}
}

// TS 36.413 §10.6
func TestReleaseCompleteWithInconsistentPairReleasesLocallyWithoutErrorIndication(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)

	HandleUEContextReleaseComplete(t.Context(), m, mme.NewRadioForTest(conn), releaseCompleteValue(t, ue.Conn().MMEUES1APID, 99))

	if conn.count() != 0 {
		t.Fatalf("sent %d messages in answer to the last message, want none", conn.count())
	}

	requireIdleAndRetained(t, m, ue)
}

// TS 36.413 §10.6
func TestReleaseCompleteFromAnotherENBReleasesNothingOnTheUEsInterface(t *testing.T) {
	m := newTestMME(t)
	ue, home := securedUE(t, m)
	foreign := &captureConn{}

	HandleUEContextReleaseComplete(t.Context(), m, mme.NewRadioForTest(foreign), releaseCompleteValue(t, ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID()))

	if foreign.count() != 0 || home.count() != 0 {
		t.Fatalf("sent %d/%d messages, want none", foreign.count(), home.count())
	}

	if !ue.Connected() || ue.Conn().ConnForTest() != home {
		t.Fatal("a Release Complete from another eNB released the UE's S1 connection")
	}
}

// TS 36.413 §10.6
func TestInitialUEMessageReusingAnAllocatedENBUES1APIDIsRejectedAndReleasesTheHolder(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)
	enbID := ue.Conn().ENBUES1APID()

	HandleInitialUEMessage(t.Context(), m, mme.NewRadioForTest(conn), initiatingValue(t, initialUEMessagePDU(t, enbID, []byte{0x07, 0x60, 0x00})))

	if conn.count() != 1 {
		t.Fatalf("sent %d messages, want only the Error Indication", conn.count())
	}

	ind := parseOutboundErrorIndication(t, conn.sent[0])

	if ind.MMEUES1APID != nil {
		t.Errorf("Error Indication carries MME-UE-S1AP-ID %d, want only the eNB-UE-S1AP-ID received", *ind.MMEUES1APID)
	}

	if ind.ENBUES1APID == nil || *ind.ENBUES1APID != enbID {
		t.Errorf("Error Indication eNB-UE-S1AP-ID = %v, want %d", ind.ENBUES1APID, enbID)
	}

	if ind.Cause == nil || *ind.Cause != causeUnknownENBUES1APID {
		t.Errorf("Error Indication cause = %v, want unknown-enb-ue-s1ap-id", ind.Cause)
	}

	requireIdleAndRetained(t, m, ue)

	if got := m.ConnCountForTest(); got != 0 {
		t.Fatalf("%d S1 connections held, want none", got)
	}
}

// TS 36.413 §10.6
func TestInitialUEMessageReusingAnENBUES1APIDOfAnotherENBIsAccepted(t *testing.T) {
	m := newTestMME(t)
	ue, home := securedUE(t, m)
	foreign := &captureConn{}

	HandleInitialUEMessage(t.Context(), m, mme.NewRadioForTest(foreign), initiatingValue(t, initialUEMessagePDU(t, ue.Conn().ENBUES1APID(), []byte{0x07, 0x60, 0x00})))

	for _, pdu := range foreign.sent {
		if isErrorIndication(t, pdu) {
			t.Fatal("an eNB-UE-S1AP-ID held on another S1 interface was treated as already allocated")
		}
	}

	if home.count() != 0 || !ue.Connected() || ue.Conn().ConnForTest() != home {
		t.Fatal("an Initial UE Message from another eNB released the UE's S1 connection")
	}
}

// TS 36.413 §10.6
func TestErrorIndicationForErroneousS1APIDsReleasesLocally(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)

	ind := &s1ap.ErrorIndication{
		MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(ue.Conn().ENBUES1APID()),
		Cause:       s1ap.Ptr(causeUnknownPairUES1APID),
	}

	handleErrorIndication(t.Context(), m, mme.NewRadioForTest(conn), initiatingValue(t, mustMarshal(t, ind.Marshal)))

	if conn.count() != 0 {
		t.Fatalf("sent %d messages for a connection the eNB has released, want none", conn.count())
	}

	requireIdleAndRetained(t, m, ue)
}

func errorIndicationsSince(t *testing.T, c *captureConn, from int) int {
	t.Helper()

	n := 0

	for _, pdu := range c.sent[from:] {
		if isErrorIndication(t, pdu) {
			n++
		}
	}

	return n
}

func heldConn(m *mme.MME, conn mme.S1APWriter, c *mme.UeConn) bool {
	for _, held := range m.ConnsOnConn(conn) {
		if held == c {
			return true
		}
	}

	return false
}

func TestInitialUEMessageReusingTheENBUES1APIDOfAConnectionBeingReleasedCompletesThatRelease(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)
	enbID := ue.Conn().ENBUES1APID()

	m.ReleaseUEContext(t.Context(), ue, mme.CauseNASNormalRelease)

	before := conn.count()

	HandleInitialUEMessage(t.Context(), m, mme.NewRadioForTest(conn), initiatingValue(t, initialUEMessagePDU(t, enbID, []byte{0x07, 0x60, 0x00})))

	if n := errorIndicationsSince(t, conn, before); n != 0 {
		t.Fatalf("sent %d Error Indications for an eNB-UE-S1AP-ID the eNB freed by completing its release, want none", n)
	}

	requireIdleAndRetained(t, m, ue)

	if !ue.MobileReachableArmedForTest() {
		t.Fatal("the implicitly completed release did not move the UE to ECM-IDLE")
	}
}

func TestInitialUEMessageReusingTheENBUES1APIDOfASupersededConnectionCompletesItsRelease(t *testing.T) {
	m := newTestMME(t)
	ue, oldConn := securedUE(t, m)

	_, oldENBID := resumeOntoNewConnection(t, m, ue)

	before := oldConn.count()

	HandleInitialUEMessage(t.Context(), m, mme.NewRadioForTest(oldConn), initiatingValue(t, initialUEMessagePDU(t, oldENBID, []byte{0x07, 0x60, 0x00})))

	if n := errorIndicationsSince(t, oldConn, before); n != 0 {
		t.Fatalf("sent %d Error Indications for the eNB-UE-S1AP-ID of a superseded connection the eNB released, want none", n)
	}

	if !ue.Connected() || ue.Conn().ENBUES1APID() != 1001 {
		t.Fatal("completing the superseded connection's release disturbed the UE's current connection")
	}
}

func TestLateReleaseCompleteLeavesTheConnectionThatReusedItsENBUES1APID(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m)
	mmeID, enbID := ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID()

	m.ReleaseUEContext(t.Context(), ue, mme.CauseNASNormalRelease)

	if !m.ClaimENBUES1APID(t.Context(), conn, enbID) {
		t.Fatal("the eNB-UE-S1AP-ID of a connection being released was not freed for reuse")
	}

	next := m.NewUeConn(conn, enbID)
	before := conn.count()

	HandleUEContextReleaseComplete(t.Context(), m, mme.NewRadioForTest(conn), releaseCompleteValue(t, mmeID, enbID))

	if conn.count() != before {
		t.Fatalf("sent %d messages in answer to the last message, want none", conn.count()-before)
	}

	if !heldConn(m, conn, next) {
		t.Fatal("a late Release Complete released the connection that reused its eNB-UE-S1AP-ID")
	}
}
