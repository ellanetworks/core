// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/tester/enb"
	"github.com/ellanetworks/core/ngap"
)

const (
	testMCC   = "001"
	testMNC   = "01"
	testTAC   = "000001"
	testEnbID = "000102"
)

func initiatingValue(t *testing.T, pdu []byte, want ngap.ProcedureCode) []byte {
	t.Helper()

	msg, err := ngap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != want {
		t.Fatalf("got %T for procedure %v, want procedure %d", msg, msg, want)
	}

	return im.Value
}

// An ng-eNB announces a macro ng-eNB ID where a gNB announces a gNB ID
// (TS 38.413 §9.3.1.5), so its NG SETUP REQUEST names a different node kind.
func TestBuildNGSetupRequestRoundTrips(t *testing.T) {
	pdu, err := enb.BuildNGSetupRequest(&enb.NGSetupRequestOpts{
		Name: "ella-ng-enb", EnbID: testEnbID,
		Mcc: testMCC, Mnc: testMNC, Tac: testTAC, Sst: 1, Sd: "010203",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseNGSetupRequest(initiatingValue(t, pdu, ngap.ProcNGSetup))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.GlobalRANNodeID.Kind != ngap.RANNodeIDMacroNgENB {
		t.Errorf("node kind = %d, want macro ng-eNB", msg.GlobalRANNodeID.Kind)
	}

	if msg.RANNodeName == nil || *msg.RANNodeName != "ella-ng-enb" {
		t.Errorf("node name = %v, want ella-ng-enb", msg.RANNodeName)
	}

	if len(msg.SupportedTAList) != 1 || msg.SupportedTAList[0].TAC != 1 {
		t.Fatalf("supported TAs = %+v, want one with TAC 1", msg.SupportedTAList)
	}

	slices := msg.SupportedTAList[0].BroadcastPLMNList[0].TAISliceSupportList
	if len(slices) != 1 || slices[0].SNSSAI.SST != 1 {
		t.Fatalf("slice support = %+v, want one with SST 1", slices)
	}

	if slices[0].SNSSAI.SD == nil || *slices[0].SNSSAI.SD != (ngap.SD{0x01, 0x02, 0x03}) {
		t.Errorf("SD = %v, want 010203", slices[0].SNSSAI.SD)
	}
}

// The ng-eNB reports an E-UTRA cell where internal/tester/gnb reports an NR one
// (TS 38.413 §9.3.1.16).
func TestBuildUplinkNasTransportRoundTrips(t *testing.T) {
	nas := []byte{0x7e, 0x00, 0x41}

	pdu, err := enb.BuildUplinkNasTransport(&enb.UplinkNasTransportOpts{
		AMFUeNgapID: 3, RANUeNgapID: 7, NasPDU: nas,
		Mcc: testMCC, Mnc: testMNC, EnbID: testEnbID, Tac: testTAC,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseUplinkNASTransport(initiatingValue(t, pdu, ngap.ProcUplinkNASTransport))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.AMFUENGAPID != 3 || msg.RANUENGAPID != 7 {
		t.Errorf("AP ids = (%d, %d), want (3, 7)", msg.AMFUENGAPID, msg.RANUENGAPID)
	}

	if string(msg.NASPDU) != string(nas) {
		t.Errorf("NAS PDU = %x, want %x", msg.NASPDU, nas)
	}

	uli := msg.UserLocationInformation
	if uli == nil || uli.Kind != ngap.UserLocationEUTRA {
		t.Fatalf("location = %+v, want an E-UTRA cell", uli)
	}

	if uli.TAI.TAC != 1 {
		t.Errorf("TAC = %d, want 1", uli.TAI.TAC)
	}
}
