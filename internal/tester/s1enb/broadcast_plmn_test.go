// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

var (
	testHomePLMN  = s1ap.PLMNIdentity{0x00, 0xf1, 0x10}
	testOtherPLMN = s1ap.PLMNIdentity{0x13, 0x00, 0x62}
)

func multiPLMNENB(t *testing.T) *ENB {
	t.Helper()

	bplmns, err := broadcastPLMNs(testHomePLMN, []nas.PLMN{{MCC: "310", MNC: "260"}})
	if err != nil {
		t.Fatalf("broadcastPLMNs: %v", err)
	}

	return &ENB{
		enbID: 0x19b, enbIDKind: s1ap.ENBIDMacro, name: "ella", plmn: testHomePLMN, bplmns: bplmns, tac: 1,
		selected: make(map[int64]s1ap.PLMNIdentity), nextENBUEID: 1,
	}
}

func initiatingValue(t *testing.T, b []byte, want s1ap.ProcedureCode) []byte {
	t.Helper()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != want {
		t.Fatalf("got %T, want an InitiatingMessage of procedure %d", pdu, want)
	}

	return im.Value
}

func TestS1SetupRequestBroadcastsAdditionalPLMNs(t *testing.T) {
	e := multiPLMNENB(t)

	b, err := e.buildS1SetupRequest()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	req, err := s1ap.ParseS1SetupRequest(initiatingValue(t, b, s1ap.ProcS1Setup))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if req.GlobalENBID.PLMNIdentity != testHomePLMN {
		t.Errorf("Global eNB ID PLMN = % x, want the primary PLMN", req.GlobalENBID.PLMNIdentity)
	}

	if len(req.SupportedTAs) != 1 {
		t.Fatalf("supported TAs = %d, want 1", len(req.SupportedTAs))
	}

	got := req.SupportedTAs[0].BroadcastPLMNs
	if len(got) != 2 || got[0] != testHomePLMN || got[1] != testOtherPLMN {
		t.Fatalf("broadcast PLMNs = % x, want primary then 310/260", got)
	}
}

func TestBroadcastPLMNsRefusesDuplicate(t *testing.T) {
	if _, err := broadcastPLMNs(testHomePLMN, []nas.PLMN{{MCC: "001", MNC: "01"}}); err == nil {
		t.Fatal("accepted the primary PLMN twice")
	}
}

func TestInitialUEMessageCarriesSelectedPLMNInTAI(t *testing.T) {
	e := multiPLMNENB(t)

	ue := e.NewUE("310260000000001", [16]byte{}, [16]byte{})

	if err := ue.SelectPLMN("999", "99"); err == nil {
		t.Fatal("SelectPLMN accepted a PLMN the eNB does not broadcast")
	}

	if err := ue.SelectPLMN("310", "260"); err != nil {
		t.Fatalf("SelectPLMN: %v", err)
	}

	if string(ue.plmn) != string(testOtherPLMN[:]) {
		t.Errorf("serving network = % x, want the selected PLMN", ue.plmn)
	}

	id := e.AllocateENBUEIDFor(ue)

	b, err := e.buildInitialUEMessage(id, []byte{0x07, 0x41})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := s1ap.ParseInitialUEMessage(initiatingValue(t, b, s1ap.ProcInitialUEMessage))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.TAI.PLMNIdentity != testOtherPLMN {
		t.Errorf("TAI PLMN = % x, want the selected 310/260", msg.TAI.PLMNIdentity)
	}

	if msg.EUTRANCGI == nil || msg.EUTRANCGI.PLMNIdentity != testHomePLMN {
		t.Errorf("E-UTRAN CGI = %+v, want the primary PLMN", msg.EUTRANCGI)
	}

	other := e.AllocateENBUEIDFor(e.NewUE("001010000000001", [16]byte{}, [16]byte{}))
	if tai := e.taiFor(other); tai.PLMNIdentity != testHomePLMN {
		t.Errorf("unselected UE TAI PLMN = % x, want the primary PLMN", tai.PLMNIdentity)
	}
}
