// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestECMIdleBuffersSession(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue).Apn = "internet"

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: 7}
	b, _ := complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), cpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.Connected() {
		t.Fatal("UE not ECM-IDLE after release complete")
	}

	if !m.Session.(*fakeSessionManager).deactivated {
		t.Fatal("EPS session not deactivated (buffered) for paging on ECM-IDLE")
	}
}

func TestUEContextReleaseRequestFromENB(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: 7,
		Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 UE Context Release Command, got %d", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])

	// A second release attempt must not emit another command (idempotent).
	m.ReleaseUEContext(context.Background(), ue, mme.CauseNASDetach)

	if len(cc.sent) != 1 {
		t.Fatalf("release not idempotent: %d commands sent", len(cc.sent))
	}

	// Completing an eNB-initiated release moves the UE to ECM-IDLE; the EMM
	// context is retained, not deleted.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: 7}

	b, _ = complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), cpdu.(*s1ap.SuccessfulOutcome).Value)

	got, ok := m.LookupUeByIMSI(ue.IMSI())
	if !ok {
		t.Fatal("EMM context deleted on an inactivity release; expected ECM-IDLE retention")
	}

	if got.Connected() {
		t.Fatal("UE not marked ECM-IDLE after eNB release")
	}

	// The released MME-UE-S1AP-ID no longer identifies an active S1 connection.
	// A repeat UE Context Release Request on the same association is answered
	// with an Error Indication, not re-actioned with another release command
	// (TS 36.413).
	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 2 {
		t.Fatalf("expected an Error Indication for the released AP ID, got %d S1AP messages", len(cc.sent))
	}

	ind := parseOutboundErrorIndication(t, cc.sent[1])
	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}

// TestUEContextReleaseRequestFromForeignENB checks that a UE-associated message
// arriving on an S1 association other than the UE's own is rejected with an
// Error Indication, not acted upon: the global MME-UE-S1AP-ID map is shared
// across eNBs, so without this an eNB could release a UE attached through
// another by presenting its AP-ID pair (TS 36.413).
func TestUEContextReleaseRequestFromForeignENB(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID,
		Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	foreign := &captureConn{}
	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(foreign), pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 0 {
		t.Fatalf("foreign eNB released a UE on another association: %d S1AP messages on the owning association", len(cc.sent))
	}

	if ue.Conn().ReleasingForTest() {
		t.Fatal("UE marked releasing by a message from a foreign association")
	}

	if len(foreign.sent) != 1 {
		t.Fatalf("expected one Error Indication to the foreign association, got %d", len(foreign.sent))
	}

	ind := parseOutboundErrorIndication(t, foreign.sent[0])
	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}
