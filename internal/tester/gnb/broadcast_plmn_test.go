// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
)

var (
	homePLMN  = ngap.PLMNIdentity{0x00, 0xf1, 0x10}
	otherPLMN = ngap.PLMNIdentity{0x13, 0x00, 0x62}
)

func TestBuildNGSetupRequestBroadcastsAdditionalPLMNs(t *testing.T) {
	pdu, err := gnb.BuildNGSetupRequest(&gnb.NGSetupRequestOpts{
		Name: "ella", GnbID: testGnbID, Mcc: testMCC, Mnc: testMNC, Tac: testTAC, Sst: 1,
		AdditionalPLMNs: []gnb.BroadcastPLMN{
			{Mcc: "310", Mnc: "260", Slices: []gnb.SliceOpt{{Sst: 2, Sd: "0a0b0c"}, {Sst: 3}}},
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseNGSetupRequest(initiatingValue(t, pdu, ngap.ProcNGSetup))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.GlobalRANNodeID.PLMNIdentity != homePLMN {
		t.Errorf("node PLMN = % x, want the primary PLMN", msg.GlobalRANNodeID.PLMNIdentity)
	}

	if len(msg.SupportedTAList) != 1 {
		t.Fatalf("supported TAs = %d, want 1", len(msg.SupportedTAList))
	}

	bplmns := msg.SupportedTAList[0].BroadcastPLMNList
	if len(bplmns) != 2 || bplmns[0].PLMNIdentity != homePLMN || bplmns[1].PLMNIdentity != otherPLMN {
		t.Fatalf("broadcast PLMNs = %+v, want primary then 310/260", bplmns)
	}

	if s := bplmns[0].TAISliceSupportList; len(s) != 1 || s[0].SNSSAI.SST != 1 {
		t.Errorf("primary slices = %+v, want SST 1", s)
	}

	s := bplmns[1].TAISliceSupportList
	if len(s) != 2 || s[0].SNSSAI.SST != 2 || s[0].SNSSAI.SD == nil || *s[0].SNSSAI.SD != (ngap.SD{0x0a, 0x0b, 0x0c}) || s[1].SNSSAI.SST != 3 {
		t.Errorf("additional slices = %+v, want SST 2/0a0b0c and SST 3", s)
	}
}

func TestBuildNGSetupRequestAdditionalPLMNInheritsSlices(t *testing.T) {
	pdu, err := gnb.BuildNGSetupRequest(&gnb.NGSetupRequestOpts{
		GnbID: testGnbID, Mcc: testMCC, Mnc: testMNC, Tac: testTAC, Sst: 1, Sd: "010203",
		AdditionalPLMNs: []gnb.BroadcastPLMN{{Mcc: "310", Mnc: "260"}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseNGSetupRequest(initiatingValue(t, pdu, ngap.ProcNGSetup))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	s := msg.SupportedTAList[0].BroadcastPLMNList[1].TAISliceSupportList
	if len(s) != 1 || s[0].SNSSAI.SST != 1 || s[0].SNSSAI.SD == nil {
		t.Errorf("slices = %+v, want the primary SST 1/010203", s)
	}
}

func TestBuildNGSetupRequestRefusesDuplicatePLMN(t *testing.T) {
	_, err := gnb.BuildNGSetupRequest(&gnb.NGSetupRequestOpts{
		GnbID: testGnbID, Mcc: testMCC, Mnc: testMNC, Tac: testTAC, Sst: 1,
		AdditionalPLMNs: []gnb.BroadcastPLMN{{Mcc: testMCC, Mnc: testMNC}},
	})
	if err == nil {
		t.Fatal("build accepted the primary PLMN twice")
	}
}

func TestBuildRANConfigurationUpdateBroadcastsAdditionalPLMNs(t *testing.T) {
	pdu, err := gnb.BuildRANConfigurationUpdate(&gnb.RANConfigurationUpdateOpts{
		Mcc: testMCC, Mnc: testMNC, Tac: testTAC, Sst: 1,
		AdditionalPLMNs: []gnb.BroadcastPLMN{{Mcc: "310", Mnc: "260"}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseRANConfigurationUpdate(initiatingValue(t, pdu, ngap.ProcRANConfigurationUpdate))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	bplmns := msg.SupportedTAList[0].BroadcastPLMNList
	if len(bplmns) != 2 || bplmns[1].PLMNIdentity != otherPLMN {
		t.Fatalf("broadcast PLMNs = %+v, want primary then 310/260", bplmns)
	}
}

func TestBuildInitialUEMessageCarriesSelectedPLMNInTAI(t *testing.T) {
	pdu, err := gnb.BuildInitialUEMessage(&gnb.InitialUEMessageOpts{
		RanUENGAPID: 1, NasPDU: []byte{0x7e}, Mcc: testMCC, Mnc: testMNC,
		SelectedMcc: "310", SelectedMnc: "260", Tac: testTAC, GnbID: testGnbID,
		RRCEstablishmentCause: ngap.RRCCauseMOSignalling,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseInitialUEMessage(initiatingValue(t, pdu, ngap.ProcInitialUEMessage))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	uli := msg.UserLocationInformation
	if uli.TAI.PLMNIdentity != otherPLMN {
		t.Errorf("TAI PLMN = % x, want the selected 310/260", uli.TAI.PLMNIdentity)
	}

	if uli.PLMNIdentity != homePLMN {
		t.Errorf("NR CGI PLMN = % x, want the primary PLMN", uli.PLMNIdentity)
	}
}

func TestBuildInitialUEMessageDefaultsTAIToServingPLMN(t *testing.T) {
	pdu, err := gnb.BuildInitialUEMessage(&gnb.InitialUEMessageOpts{
		RanUENGAPID: 1, NasPDU: []byte{0x7e}, Mcc: testMCC, Mnc: testMNC, Tac: testTAC, GnbID: testGnbID,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseInitialUEMessage(initiatingValue(t, pdu, ngap.ProcInitialUEMessage))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.UserLocationInformation.TAI.PLMNIdentity != homePLMN {
		t.Errorf("TAI PLMN = % x, want the primary PLMN", msg.UserLocationInformation.TAI.PLMNIdentity)
	}
}

func TestBuildUplinkNasTransportCarriesSelectedPLMNInTAI(t *testing.T) {
	pdu, err := gnb.BuildUplinkNasTransport(&gnb.UplinkNasTransportOpts{
		AMFUeNgapID: 2, RANUeNgapID: 1, NasPDU: []byte{0x7e}, Mcc: testMCC, Mnc: testMNC,
		SelectedMcc: "310", SelectedMnc: "260", Tac: testTAC, GnbID: testGnbID,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseUplinkNASTransport(initiatingValue(t, pdu, ngap.ProcUplinkNASTransport))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.UserLocationInformation == nil || msg.UserLocationInformation.TAI.PLMNIdentity != otherPLMN {
		t.Errorf("location = %+v, want TAI PLMN 310/260", msg.UserLocationInformation)
	}
}

func TestSelectPLMNAcceptsOnlyBroadcastPLMNs(t *testing.T) {
	g := &gnb.GnodeB{MCC: testMCC, MNC: testMNC, AdditionalPLMNs: []gnb.BroadcastPLMN{{Mcc: "310", Mnc: "260"}}}

	if err := g.SelectPLMN(5, "999", "99"); err == nil {
		t.Fatal("SelectPLMN accepted a PLMN the node does not broadcast")
	}

	if mcc, mnc := g.SelectedPLMN(5); mcc != testMCC || mnc != testMNC {
		t.Fatalf("unselected UE = %s/%s, want the primary PLMN", mcc, mnc)
	}

	if err := g.SelectPLMN(5, "310", "260"); err != nil {
		t.Fatalf("SelectPLMN: %v", err)
	}

	if mcc, mnc := g.SelectedPLMN(5); mcc != "310" || mnc != "260" {
		t.Fatalf("selected = %s/%s, want 310/260", mcc, mnc)
	}

	if mcc, _ := g.SelectedPLMN(6); mcc != testMCC {
		t.Fatalf("another UE = %s, want the primary PLMN", mcc)
	}
}
