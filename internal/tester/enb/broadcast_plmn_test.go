// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/tester/enb"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
)

func TestBuildNGSetupRequestBroadcastsAdditionalPLMNs(t *testing.T) {
	pdu, err := enb.BuildNGSetupRequest(&enb.NGSetupRequestOpts{
		EnbID: testEnbID, Mcc: testMCC, Mnc: testMNC, Tac: testTAC, Sst: 1,
		AdditionalPLMNs: []gnb.BroadcastPLMN{{Mcc: "310", Mnc: "260", Slices: []gnb.SliceOpt{{Sst: 2}}}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseNGSetupRequest(initiatingValue(t, pdu, ngap.ProcNGSetup))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	bplmns := msg.SupportedTAList[0].BroadcastPLMNList
	if len(bplmns) != 2 || bplmns[1].PLMNIdentity != (ngap.PLMNIdentity{0x13, 0x00, 0x62}) {
		t.Fatalf("broadcast PLMNs = %+v, want primary then 310/260", bplmns)
	}

	if s := bplmns[1].TAISliceSupportList; len(s) != 1 || s[0].SNSSAI.SST != 2 {
		t.Errorf("additional slices = %+v, want SST 2", s)
	}
}

func TestBuildInitialUEMessageCarriesSelectedPLMNInTAI(t *testing.T) {
	pdu, err := enb.BuildInitialUEMessage(&enb.InitialUEMessageOpts{
		RanUENGAPID: 1, NasPDU: []byte{0x7e}, Mcc: testMCC, Mnc: testMNC,
		SelectedMcc: "310", SelectedMnc: "260", Tac: testTAC, EnbID: testEnbID,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseInitialUEMessage(initiatingValue(t, pdu, ngap.ProcInitialUEMessage))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	uli := msg.UserLocationInformation
	if uli.Kind != ngap.UserLocationEUTRA || uli.TAI.PLMNIdentity != (ngap.PLMNIdentity{0x13, 0x00, 0x62}) {
		t.Errorf("location = %+v, want an E-UTRA cell with TAI PLMN 310/260", uli)
	}

	if uli.PLMNIdentity != (ngap.PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Errorf("E-UTRA CGI PLMN = % x, want the primary PLMN", uli.PLMNIdentity)
	}
}
