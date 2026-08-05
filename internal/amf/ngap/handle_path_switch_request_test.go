// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

func buildPathSwitchRequestTransfer(teid uint32, ip []byte) (ngap.TransferContainer, error) {
	return (&ngap.PathSwitchRequestTransfer{
		DLNGUUPTNLInformation: ngap.UPTransportLayerInformation{GTPTunnel: ngap.GTPTunnel{
			TransportLayerAddress: ngap.TransportLayerAddress(ip),
			GTPTEID:               ngap.GTPTEID(teid),
		}},
		// QosFlowAcceptedList is mandatory.
		QosFlowAccepted: ngap.QosFlowAcceptedList{{QosFlowIdentifier: 1}},
	}).Marshal()
}

// buildPathSwitchRequest assembles a PATH SWITCH REQUEST. UserLocationInformation
// is always set: it is mandatory, and the handler reads it to update the UE's
// recorded location.
func buildPathSwitchRequest(
	sourceAmfUeNgapID *ngap.AMFUENGAPID,
	ranUeNgapID *ngap.RANUENGAPID,
	pduSessionDLList ngap.PDUSessionResourceToBeSwitchedDLList,
	failedList ngap.PDUSessionResourceFailedToSetupListPSReq,
	uESecurityCapabilities *ngap.UESecurityCapabilities,
) *ngap.PathSwitchRequest {
	msg := &ngap.PathSwitchRequest{
		UserLocationInformation: &ngap.UserLocationInformation{
			Kind:         ngap.UserLocationNR,
			PLMNIdentity: ngap.PLMNIdentity{0x02, 0xf8, 0x39},
			CellIdentity: 1,
			TAI:          ngap.TAI{PLMNIdentity: ngap.PLMNIdentity{0x02, 0xf8, 0x39}, TAC: 1},
		},
		UESecurityCapabilities:               uESecurityCapabilities,
		PDUSessionResourceToBeSwitchedDLList: pduSessionDLList,
		PDUSessionResourceFailedToSetup:      failedList,
	}

	if sourceAmfUeNgapID != nil {
		msg.SourceAMFUENGAPID = *sourceAmfUeNgapID
	}

	if ranUeNgapID != nil {
		msg.RANUENGAPID = *ranUeNgapID
	}

	return msg
}

func newTestAMFWithSmf(smf amf.SmfSbi) *amf.AMF {
	return amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, smf)
}

func newValidUeContext() *amf.UeContext {
	amfUe := amf.NewUeContext()
	supi, _ := etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")
	amfUe.SetNHForTest(make([]byte, 32))

	amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})

	return amfUe
}

func TestPathSwitchRequest_UnknownUE(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}

	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	ran.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(999)), // no UE with this AMF UE NGAP ID
		ngap.Ptr(ngap.RANUENGAPID(1)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(sender.SentPathSwitchRequestFailures))
	}

	failure := sender.SentPathSwitchRequestFailures[0]
	if failure.AMFUENGAPID == nil || *failure.AMFUENGAPID != ngap.AMFUENGAPID(999) {
		t.Errorf("expected AmfUeNgapID=999, got %d", failure.AMFUENGAPID)
	}

	if failure.RANUENGAPID == nil || *failure.RANUENGAPID != ngap.RANUENGAPID(1) {
		t.Errorf("expected RanUeNgapID=1, got %d", failure.RANUENGAPID)
	}
}

func TestPathSwitchRequest_NilUeContext(t *testing.T) {
	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	// UeConn exists but UeContext is nil
	amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_InvalidSecurityContext(t *testing.T) {
	sender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}

	amfUe := amf.NewUeContext()
	amfUe.SetSecuredForTest(false)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 7}) // ngKSI "no key is available" (TS 24.501 §9.11.3.32)

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_SmContextNotFound(t *testing.T) {
	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	// SmContextList is empty — no PDU session ID 1

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0x01}}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(fakeSmf.PathSwitchCalls) != 0 {
		t.Fatalf("expected no SMF calls, got %d", len(fakeSmf.PathSwitchCalls))
	}

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}

	// TS 38.413: the failure must name the unswitched session in its
	// mandatory PDU Session Resource Released List.
	released := targetNGAPSender.SentPathSwitchRequestFailures[0].PDUSessionResourceReleased
	if released == nil || len(released) != 1 {
		t.Fatalf("failure must carry a released list naming the unswitched session (TS 38.413); got %v", released)
	}

	if released[0].PDUSessionID != 1 {
		t.Errorf("released PDU session ID = %d, want 1", released[0].PDUSessionID)
	}
}

func TestPathSwitchRequest_SmfReturnsError(t *testing.T) {
	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	fakeSmf := &fakeSmfSbi{
		PathSwitchErr: fmt.Errorf("PFCP modification failed"),
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 SMF PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	if fakeSmf.PathSwitchCalls[0].SmContextRef != "imsi-001010000000001-1" {
		t.Errorf("expected SmContextRef=imsi-001010000000001-1, got %s", fakeSmf.PathSwitchCalls[0].SmContextRef)
	}

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 0 {
		t.Fatalf("expected no PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}
}

func TestPathSwitchRequest_HappyPath(t *testing.T) {
	const (
		pduSessionID      = uint8(1)
		sourceAmfUeNgapID = int64(10)
		targetRanUeNgapID = int64(2)
		kamfHex           = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, models.AmfUeNgapID(sourceAmfUeNgapID), logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	n2Response := []byte{0xAA, 0xBB, 0xCC}
	fakeSmf := &fakeSmfSbi{
		PathSwitchResponse: n2Response,
	}

	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(sourceAmfUeNgapID)),
		ngap.Ptr(ngap.RANUENGAPID(targetRanUeNgapID)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 SMF PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	if fakeSmf.PathSwitchCalls[0].SmContextRef != "imsi-001010000000001-1" {
		t.Errorf("expected SmContextRef=imsi-001010000000001-1, got %s", fakeSmf.PathSwitchCalls[0].SmContextRef)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]

	if ack.AMFUENGAPID == nil || *ack.AMFUENGAPID != ngap.AMFUENGAPID(sourceAmfUeNgapID) {
		t.Errorf("expected AmfUeNgapID=%d, got %d", sourceAmfUeNgapID, ack.AMFUENGAPID)
	}

	if ack.RANUENGAPID == nil || *ack.RANUENGAPID != ngap.RANUENGAPID(targetRanUeNgapID) {
		t.Errorf("expected RanUeNgapID=%d, got %d", targetRanUeNgapID, ack.RANUENGAPID)
	}

	if ack.SecurityContext.NextHopChainingCount == 0 {
		t.Error("expected NCC > 0 after advancing the NH chain")
	}

	if len(ack.PDUSessionResourceSwitchedList) != 1 {
		t.Fatalf("expected 1 switched PDU session, got %d", len(ack.PDUSessionResourceSwitchedList))
	}

	if ack.PDUSessionResourceSwitchedList[0].PDUSessionID != ngap.PDUSessionID(pduSessionID) {
		t.Errorf("expected PDU session ID %d, got %d", pduSessionID, ack.PDUSessionResourceSwitchedList[0].PDUSessionID)
	}

	if sourceUe.Radio() != targetRan {
		t.Error("expected UeConn to be switched to targetRan")
	}

	if sourceUe.RanUeNgapID != models.RanUeNgapID(targetRanUeNgapID) {
		t.Errorf("expected RanUeNgapID=%d, got %d", targetRanUeNgapID, sourceUe.RanUeNgapID)
	}

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 0 {
		t.Fatalf("expected no PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

// TestPathSwitchRequest_RejectedWhileKeyChainBusy verifies the {NH,NCC} chain claim
// (TS 33.501 §6.9.5): while an N2 handover holds the key chain, a path switch is
// rejected with a Path Switch Failure — without touching the SMF or advancing the
// chain — so the two cannot double-advance the one {NH,NCC} counter.
func TestPathSwitchRequest_RejectedWhileKeyChainBusy(t *testing.T) {
	const (
		pduSessionID      = uint8(1)
		sourceAmfUeNgapID = int64(10)
		targetRanUeNgapID = int64(2)
		kamfHex           = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: sourceNGAPSender}

	amfUe := newValidUeContext()
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, models.AmfUeNgapID(sourceAmfUeNgapID), logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{Log: logger.AmfLog, Conn: targetNGAPSender}

	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA, 0xBB, 0xCC}}

	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	// A concurrent N2 handover holds the key chain.
	if err := amfUe.Procedures().Begin(procedure.N2Handover); err != nil {
		t.Fatalf("failed to begin N2Handover: %v", err)
	}

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(sourceAmfUeNgapID)),
		ngap.Ptr(ngap.RANUENGAPID(targetRanUeNgapID)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d", len(targetNGAPSender.SentPathSwitchRequestFailures))
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 0 {
		t.Fatalf("expected no PathSwitchRequestAcknowledge, got %d", len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	// The SMF must not be touched: a rejected path switch changes nothing.
	if len(fakeSmf.PathSwitchCalls) != 0 {
		t.Fatalf("expected no SMF PathSwitch call on a rejected path switch, got %d", len(fakeSmf.PathSwitchCalls))
	}

	// The UE stays on its source RAN; the key chain was not advanced.
	if sourceUe.Radio() != sourceRan {
		t.Error("expected UeConn to stay on sourceRan after a rejected path switch")
	}
}

// TestPathSwitchRequest_DuplicatePDUSessionIDs verifies TS 38.413: a
// to-be-switched downlink list that repeats a PDU Session ID is rejected with a
// Path Switch Request Failure, even for an otherwise-switchable UE context, and
// neither the SMF nor an acknowledge is invoked.
func TestPathSwitchRequest_DuplicatePDUSessionIDs(t *testing.T) {
	const (
		pduSessionID      = uint8(1)
		sourceAmfUeNgapID = int64(10)
		targetRanUeNgapID = int64(2)
		kamfHex           = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, models.AmfUeNgapID(sourceAmfUeNgapID), logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA, 0xBB, 0xCC}}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	// PDU Session ID 1 listed twice.
	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(sourceAmfUeNgapID)),
		ngap.Ptr(ngap.RANUENGAPID(targetRanUeNgapID)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: transfer},
			{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d", len(targetNGAPSender.SentPathSwitchRequestFailures))
	}

	failure := targetNGAPSender.SentPathSwitchRequestFailures[0]
	if failure.AMFUENGAPID == nil || *failure.AMFUENGAPID != ngap.AMFUENGAPID(sourceAmfUeNgapID) {
		t.Errorf("expected AmfUeNgapID=%d, got %d", sourceAmfUeNgapID, failure.AMFUENGAPID)
	}

	if failure.RANUENGAPID == nil || *failure.RANUENGAPID != ngap.RANUENGAPID(targetRanUeNgapID) {
		t.Errorf("expected RanUeNgapID=%d, got %d", targetRanUeNgapID, failure.RANUENGAPID)
	}

	// The duplicated PDU Session ID appears once in the mandatory released list
	// (TS 38.413).
	if failure.PDUSessionResourceReleased == nil || len(failure.PDUSessionResourceReleased) != 1 {
		t.Fatalf("failure must carry a deduplicated released list (TS 38.413); got %v", failure.PDUSessionResourceReleased)
	}

	if got := failure.PDUSessionResourceReleased[0].PDUSessionID; got != ngap.PDUSessionID(pduSessionID) {
		t.Errorf("released PDU session ID = %d, want %d", got, pduSessionID)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 0 {
		t.Errorf("expected no PathSwitchRequestAcknowledge, got %d", len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	if len(fakeSmf.PathSwitchCalls) != 0 {
		t.Errorf("expected no SMF PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}
}

func TestPathSwitchRequest_MultiplePDUSessions_PartialSuccess(t *testing.T) {
	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	// Session 1 has a context, session 2 does not
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	fakeSmf := &fakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer1, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer1: %v", err)
	}

	transfer2, err := buildPathSwitchRequestTransfer(6000, []byte{10, 0, 0, 3})
	if err != nil {
		t.Fatalf("failed to build transfer2: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer1},
			{PDUSessionID: 2, Transfer: transfer2}, // No SmContext for this
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 SMF PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	// A path switch with at least one switched session is acknowledged, not failed.
	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]
	if len(ack.PDUSessionResourceSwitchedList) != 1 {
		t.Fatalf("expected 1 switched PDU session, got %d",
			len(ack.PDUSessionResourceSwitchedList))
	}

	if ack.PDUSessionResourceSwitchedList[0].PDUSessionID != 1 {
		t.Errorf("expected PDU session 1 to be switched, got %d",
			ack.PDUSessionResourceSwitchedList[0].PDUSessionID)
	}
}

func TestPathSwitchRequest_FailedPDUSessionsReportedToSmf(t *testing.T) {
	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}
	amfUe.SmContextList[2] = &amf.SmContext{
		Ref:    "imsi-001010000000001-2",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	fakeSmf := &fakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	failedBytes, err := (&ngap.PathSwitchRequestSetupFailedTransfer{
		Cause: ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID},
	}).Marshal()
	if err != nil {
		t.Fatalf("failed to marshal failed transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		ngap.PDUSessionResourceFailedToSetupListPSReq{
			{PDUSessionID: 2, Transfer: failedBytes},
		},
		nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	if len(fakeSmf.HandoverFailedCalls) != 1 {
		t.Fatalf("expected 1 HandoverFailed call, got %d", len(fakeSmf.HandoverFailedCalls))
	}

	if fakeSmf.HandoverFailedCalls[0].SmContextRef != "imsi-001010000000001-2" {
		t.Errorf("expected SmContextRef=imsi-001010000000001-2, got %s", fakeSmf.HandoverFailedCalls[0].SmContextRef)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}
}

// TestPathSwitchRequest_UESecurityCapabilitiesNotOverwritten verifies the
// AMF keeps its stored UE 5G security capabilities when the target gNB
// reports different values in a PathSwitchRequest (TS 33.501).
func TestPathSwitchRequest_UESecurityCapabilitiesNotOverwritten(t *testing.T) {
	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	amfUe.SetUESecurityCapabilityForTest(secCapFromBytes(0x70, 0x70, 0x00, 0x00))
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	fakeSmf := &fakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	secCap := &ngap.UESecurityCapabilities{}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		nil,
		secCap,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if got := eaBit(amfUe.UESecurityCapabilityForTest(), 1); got != 1 {
		t.Errorf("stored EA1_128_5G was overwritten: got %d, want 1", got)
	}

	if got := eaBit(amfUe.UESecurityCapabilityForTest(), 2); got != 1 {
		t.Errorf("stored EA2_128_5G was overwritten: got %d, want 1", got)
	}

	if got := eaBit(amfUe.UESecurityCapabilityForTest(), 3); got != 1 {
		t.Errorf("stored EA3_128_5G was overwritten: got %d, want 1", got)
	}

	if got := iaBit(amfUe.UESecurityCapabilityForTest(), 1); got != 1 {
		t.Errorf("stored IA1_128_5G was overwritten: got %d, want 1", got)
	}

	if got := iaBit(amfUe.UESecurityCapabilityForTest(), 2); got != 1 {
		t.Errorf("stored IA2_128_5G was overwritten: got %d, want 1", got)
	}

	if got := iaBit(amfUe.UESecurityCapabilityForTest(), 3); got != 1 {
		t.Errorf("stored IA3_128_5G was overwritten: got %d, want 1", got)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]
	if ack.UESecurityCapabilities == nil {
		t.Fatal("PathSwitchRequestAcknowledge has nil UESecurityCapability")
	}

	parsedCap := ngapToNasUESecurityCapability(ack.UESecurityCapabilities)
	if !parsedCap.SupportsEA(1) || !parsedCap.SupportsEA(2) || !parsedCap.SupportsEA(3) ||
		!parsedCap.SupportsIA(1) || !parsedCap.SupportsIA(2) || !parsedCap.SupportsIA(3) {
		t.Error("PathSwitchRequestAcknowledge does not echo locally stored UE security capabilities")
	}
}

// TestPathSwitchRequest_UESecurityCapabilitiesMatching exercises the
// happy path where the target gNB reports the same capabilities the AMF
// has stored.
func TestPathSwitchRequest_UESecurityCapabilitiesMatching(t *testing.T) {
	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	amfUe := newValidUeContext()
	amfUe.SetUESecurityCapabilityForTest(secCapFromBytes(0x40, 0x20, 0x00, 0x00))
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	fakeSmf := &fakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	matchingCaps := &ngap.UESecurityCapabilities{
		NREncryptionAlgorithms:          0x8000, // EA1
		NRIntegrityProtectionAlgorithms: 0x4000, // IA2
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: transfer},
		},
		nil,
		matchingCaps,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if got := eaBit(amfUe.UESecurityCapabilityForTest(), 1); got != 1 {
		t.Errorf("stored EA1_128_5G changed after matching path switch: got %d, want 1", got)
	}

	if got := iaBit(amfUe.UESecurityCapabilityForTest(), 2); got != 1 {
		t.Errorf("stored IA2_128_5G changed after matching path switch: got %d, want 1", got)
	}

	if got := eaBit(amfUe.UESecurityCapabilityForTest(), 2); got != 0 {
		t.Errorf("stored EA2_128_5G unexpectedly set: got %d, want 0", got)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]
	if ack.UESecurityCapabilities == nil {
		t.Fatal("PathSwitchRequestAcknowledge has nil UESecurityCapability")
	}

	parsedCap := ngapToNasUESecurityCapability(ack.UESecurityCapabilities)
	if !parsedCap.SupportsEA(1) || !parsedCap.SupportsIA(2) {
		t.Error("PathSwitchRequestAcknowledge does not echo locally stored UE security capabilities")
	}
}

// TestPathSwitchRequest_EmptySecurityCapabilityBytes covers a
// PathSwitchRequest whose UESecurityCapabilities IE has empty NR
// bitstrings: the handler must not panic, must leave stored capabilities
// The in-house UESecurityCapabilities carries four plain uint16 algorithm
// masks, so the empty-bitstring case the reference codec could represent is
// unrepresentable here and needs no test.

func TestPathSwitchRequest_PartialFailureReleasesUnswitched(t *testing.T) {
	const (
		switchedID        = uint8(1)
		unswitchedID      = uint8(2)
		sourceAmfUeNgapID = int64(10)
		targetRanUeNgapID = int64(2)
		kamfHex           = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: sourceNGAPSender}

	amfUe := newValidUeContext()
	amfUe.SetKamfForTest(kamfHex)
	// Only the switched session has an SM context; the other resolves to
	// SmContext-not-found and must be reported in the released list.
	amfUe.SmContextList[switchedID] = &amf.SmContext{Ref: "imsi-001010000000001-1", Snssai: &models.Snssai{Sst: 1}}

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, models.AmfUeNgapID(sourceAmfUeNgapID), logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{Log: logger.AmfLog, Conn: targetNGAPSender}

	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA, 0xBB, 0xCC}}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(sourceAmfUeNgapID)),
		ngap.Ptr(ngap.RANUENGAPID(targetRanUeNgapID)),
		ngap.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: ngap.PDUSessionID(switchedID), Transfer: transfer},
			{PDUSessionID: ngap.PDUSessionID(unswitchedID), Transfer: transfer},
		},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d", len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]

	if len(ack.PDUSessionResourceSwitchedList) != 1 {
		t.Fatalf("expected 1 switched PDU session, got %d", len(ack.PDUSessionResourceSwitchedList))
	}

	if len(ack.PDUSessionResourceReleased) != 1 {
		t.Fatalf("expected 1 released PDU session (TS 38.413 §8.4.4.2), got %d", len(ack.PDUSessionResourceReleased))
	}

	if got := ack.PDUSessionResourceReleased[0].PDUSessionID; got != ngap.PDUSessionID(unswitchedID) {
		t.Fatalf("released PDU session = %d, want %d", got, unswitchedID)
	}
}

func eaBit(sc *fgs.UESecurityCapability, n uint8) uint8 {
	if !sc.SupportsEA(n) {
		return 0
	}

	return 1
}

func iaBit(sc *fgs.UESecurityCapability, n uint8) uint8 {
	if !sc.SupportsIA(n) {
		return 0
	}

	return 1
}
