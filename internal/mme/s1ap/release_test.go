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

	m.ReleaseUEContext(context.Background(), ue, mme.CauseNASNormalRelease)

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID), ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7))}
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

func TestUEContextReleaseCompleteCapturesLocation(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.ReleaseUEContext(context.Background(), ue, mme.CauseNASNormalRelease)

	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}
	complete := &s1ap.UEContextReleaseComplete{
		MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
		UserLocationInformation: &s1ap.UserLocationInformation{
			EUTRANCGI: s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 0x0abcde1},
			TAI:       s1ap.TAI{PLMNIdentity: plmn, TAC: 9},
		},
	}
	b, _ := complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), cpdu.(*s1ap.SuccessfulOutcome).Value)

	loc := ue.GetUserLocation()
	if loc.EutraLocation == nil || loc.EutraLocation.Ecgi.EutraCellID != "0abcde1" {
		t.Fatalf("serving cell not captured from Release Complete ULI: %+v", loc.EutraLocation)
	}
}

func TestUEContextReleaseRequestFromENB(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: 7,
		Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}),
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 UE Context Release Command, got %d", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])

	m.ReleaseUEContext(context.Background(), ue, mme.CauseNASDetach)

	if len(cc.sent) != 1 {
		t.Fatalf("release not idempotent: %d commands sent", len(cc.sent))
	}

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID), ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7))}

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

	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 2 {
		t.Fatalf("expected an Error Indication for the released AP ID, got %d S1AP messages", len(cc.sent))
	}

	ind := parseOutboundErrorIndication(t, cc.sent[1])
	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}

func TestUEContextReleaseRequestFromForeignENB(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID,
		Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}),
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	foreign := &captureConn{}
	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(foreign), pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 0 {
		t.Fatalf("foreign eNB released a UE on another association: %d S1AP messages on the owning association", len(cc.sent))
	}

	if ue.ReleasingForTest() {
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
