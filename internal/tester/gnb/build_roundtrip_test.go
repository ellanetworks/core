// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb_test

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
)

const (
	testMCC   = "001"
	testMNC   = "01"
	testTAC   = "000001"
	testGnbID = "000102"
)

// initiatingValue unwraps a built PDU the way the AMF's dispatcher does, so a
// builder that names the wrong procedure fails here rather than at the peer.
func initiatingValue(t *testing.T, pdu []byte, want ngap.ProcedureCode) []byte {
	t.Helper()

	msg, err := ngap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*ngap.InitiatingMessage)
	if !ok {
		t.Fatalf("got %T, want an InitiatingMessage", msg)
	}

	if im.ProcedureCode != want {
		t.Fatalf("procedure = %d, want %d", im.ProcedureCode, want)
	}

	return im.Value
}

func successfulValue(t *testing.T, pdu []byte, want ngap.ProcedureCode) []byte {
	t.Helper()

	msg, err := ngap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := msg.(*ngap.SuccessfulOutcome)
	if !ok {
		t.Fatalf("got %T, want a SuccessfulOutcome", msg)
	}

	if so.ProcedureCode != want {
		t.Fatalf("procedure = %d, want %d", so.ProcedureCode, want)
	}

	return so.Value
}

// The User Location Information every UE-associated uplink message carries must
// name the cell this simulator serves, in the PLMN and TAI it announced
// (TS 38.413 §9.3.1.16).
func assertUserLocation(t *testing.T, uli *ngap.UserLocationInformation) {
	t.Helper()

	if uli == nil {
		t.Fatal("no User Location Information")
	}

	if uli.Kind != ngap.UserLocationNR {
		t.Errorf("location kind = %d, want NR", uli.Kind)
	}

	wantPLMN := ngap.PLMNIdentity{0x00, 0xf1, 0x10} // 001/01
	if uli.PLMNIdentity != wantPLMN || uli.TAI.PLMNIdentity != wantPLMN {
		t.Errorf("PLMN = %x / TAI %x, want %x", uli.PLMNIdentity, uli.TAI.PLMNIdentity, wantPLMN)
	}

	if uli.TAI.TAC != 1 {
		t.Errorf("TAC = %d, want 1", uli.TAI.TAC)
	}

	node, err := gnb.GNBNodeID(testMCC, testMNC, testGnbID)
	if err != nil {
		t.Fatal(err)
	}

	wantCell, err := node.NRCellIdentity(0)
	if err != nil {
		t.Fatal(err)
	}

	if uli.CellIdentity != wantCell {
		t.Errorf("cell identity = %#x, want %#x", uli.CellIdentity, wantCell)
	}
}

func TestBuildUplinkNasTransportRoundTrips(t *testing.T) {
	nas := []byte{0x7e, 0x00, 0x41}

	pdu, err := gnb.BuildUplinkNasTransport(&gnb.UplinkNasTransportOpts{
		AMFUeNgapID: 3, RANUeNgapID: 7, NasPDU: nas,
		Mcc: testMCC, Mnc: testMNC, GnbID: testGnbID, Tac: testTAC,
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

	assertUserLocation(t, msg.UserLocationInformation)
}

func TestBuildHandoverNotifyRoundTrips(t *testing.T) {
	pdu, err := gnb.BuildHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: 11, RANUENGAPID: 12,
		Mcc: testMCC, Mnc: testMNC, GnbID: testGnbID, Tac: testTAC,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseHandoverNotify(initiatingValue(t, pdu, ngap.ProcHandoverNotification))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.AMFUENGAPID != 11 || msg.RANUENGAPID != 12 {
		t.Errorf("AP ids = (%d, %d), want (11, 12)", msg.AMFUENGAPID, msg.RANUENGAPID)
	}

	assertUserLocation(t, msg.UserLocationInformation)
}

func TestBuildPathSwitchRequestRoundTrips(t *testing.T) {
	var sessions [16]*gnb.PDUSessionInformation

	sessions[1] = &gnb.PDUSessionInformation{PDUSessionID: 1, DLTeid: 0x11223344}

	pdu, err := gnb.BuildPathSwitchRequest(&gnb.PathSwitchRequestOpts{
		RANUENGAPID: 5, SourceAMFUENGAPID: 9,
		PDUSessions: sessions,
		N3GnbIp:     netip.MustParseAddr("10.0.0.2"),
		Mcc:         testMCC, Mnc: testMNC, GnbID: testGnbID, Tac: testTAC,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParsePathSwitchRequest(initiatingValue(t, pdu, ngap.ProcPathSwitchRequest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.RANUENGAPID != 5 || msg.SourceAMFUENGAPID != 9 {
		t.Errorf("AP ids = (%d, %d), want (5, 9)", msg.RANUENGAPID, msg.SourceAMFUENGAPID)
	}

	assertUserLocation(t, msg.UserLocationInformation)

	if len(msg.PDUSessionResourceToBeSwitchedDLList) != 1 {
		t.Fatalf("switched sessions = %d, want 1", len(msg.PDUSessionResourceToBeSwitchedDLList))
	}

	item := msg.PDUSessionResourceToBeSwitchedDLList[0]
	if item.PDUSessionID != 1 {
		t.Errorf("PDU session id = %d, want 1", item.PDUSessionID)
	}

	transfer, err := ngap.ParsePathSwitchRequestTransfer(item.Transfer)
	if err != nil {
		t.Fatalf("parse transfer: %v", err)
	}

	if transfer.DLNGUUPTNLInformation.GTPTunnel.GTPTEID != 0x11223344 {
		t.Errorf("TEID = %#x, want 0x11223344", transfer.DLNGUUPTNLInformation.GTPTunnel.GTPTEID)
	}

	if got := transfer.DLNGUUPTNLInformation.GTPTunnel.TransportLayerAddress; string(got) != string([]byte{10, 0, 0, 2}) {
		t.Errorf("transport address = %x, want 0a000002", got)
	}

	if len(transfer.QosFlowAccepted) != 1 {
		t.Errorf("accepted QoS flows = %d, want 1", len(transfer.QosFlowAccepted))
	}
}

func TestBuildPDUSessionResourceSetupResponseRoundTrips(t *testing.T) {
	var sessions [16]*gnb.PDUSessionInformation

	sessions[1] = &gnb.PDUSessionInformation{
		PDUSessionID: 1, DLTeid: 0xdeadbeef, QFI: 1,
		N3GnbIp: netip.MustParseAddr("10.0.0.2"),
	}

	pdu, err := gnb.BuildPDUSessionResourceSetupResponse(&gnb.PDUSessionResourceSetupResponseOpts{
		AMFUENGAPID: 1, RANUENGAPID: 2, PDUSessions: sessions,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParsePDUSessionResourceSetupResponse(successfulValue(t, pdu, ngap.ProcPDUSessionResourceSetup))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(msg.PDUSessionResourceSetup) != 1 {
		t.Fatalf("set-up sessions = %d, want 1", len(msg.PDUSessionResourceSetup))
	}

	transfer, err := ngap.ParsePDUSessionResourceSetupResponseTransfer(msg.PDUSessionResourceSetup[0].Transfer)
	if err != nil {
		t.Fatalf("parse transfer: %v", err)
	}

	if transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.GTPTEID != 0xdeadbeef {
		t.Errorf("TEID = %#x, want 0xdeadbeef", transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.GTPTEID)
	}

	if len(transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList) != 1 {
		t.Errorf("associated QoS flows = %d, want 1", len(transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList))
	}
}

// The modify and release responses carry a transfer with no mandatory field
// (TS 38.413 §9.3.4.4, §9.3.4.21), so this pins that an empty one is emitted
// and reads back.
func TestBuildPDUSessionResponsesRoundTrip(t *testing.T) {
	t.Run("modify", func(t *testing.T) {
		pdu, err := gnb.BuildPDUSessionResourceModifyResponse(&gnb.PDUSessionResourceModifyResponseOpts{
			AMFUENGAPID: 1, RANUENGAPID: 2, PDUSessionIDs: []int64{5},
		})
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		msg, err := ngap.ParsePDUSessionResourceModifyResponse(successfulValue(t, pdu, ngap.ProcPDUSessionResourceModify))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		if len(msg.PDUSessionResourceModify) != 1 || msg.PDUSessionResourceModify[0].PDUSessionID != 5 {
			t.Fatalf("modified sessions = %+v, want one with id 5", msg.PDUSessionResourceModify)
		}

		if _, err := ngap.ParsePDUSessionResourceModifyResponseTransfer(msg.PDUSessionResourceModify[0].Transfer); err != nil {
			t.Errorf("parse transfer: %v", err)
		}
	})

	t.Run("release", func(t *testing.T) {
		pdu, err := gnb.BuildPDUSessionResourceReleaseResponse(&gnb.PDUSessionResourceReleaseResponseOpts{
			AMFUENGAPID: 1, RANUENGAPID: 2, PDUSessionIDs: []int64{6},
		})
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		msg, err := ngap.ParsePDUSessionResourceReleaseResponse(successfulValue(t, pdu, ngap.ProcPDUSessionResourceRelease))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		if len(msg.PDUSessionResourceReleased) != 1 || msg.PDUSessionResourceReleased[0].PDUSessionID != 6 {
			t.Fatalf("released sessions = %+v, want one with id 6", msg.PDUSessionResourceReleased)
		}

		if _, err := ngap.ParsePDUSessionResourceReleaseResponseTransfer(msg.PDUSessionResourceReleased[0].Transfer); err != nil {
			t.Errorf("parse transfer: %v", err)
		}
	})
}

func TestBuildUplinkRANStatusTransferRoundTrips(t *testing.T) {
	pdu, err := gnb.BuildUplinkRANStatusTransfer(&gnb.UplinkRANStatusTransferOpts{
		AMFUENGAPID: 7,
		RANUENGAPID: 11,
		Container:   []byte{0x5A, 0x71, 0x03, 0x11},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := ngap.ParseUplinkRANStatusTransfer(initiatingValue(t, pdu, ngap.ProcUplinkRANStatusTransfer))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.AMFUENGAPID != 7 || msg.RANUENGAPID != 11 {
		t.Errorf("AP ids = (%d, %d), want (7, 11)", msg.AMFUENGAPID, msg.RANUENGAPID)
	}

	if string(msg.Container) != string([]byte{0x5A, 0x71, 0x03, 0x11}) {
		t.Errorf("container = %x, want 5a710311", msg.Container)
	}
}
