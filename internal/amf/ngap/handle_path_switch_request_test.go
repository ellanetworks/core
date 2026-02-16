// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"fmt"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
)

func buildPathSwitchRequestTransfer(teid uint32, ip []byte) ([]byte, error) {
	transfer := ngapType.PathSwitchRequestTransfer{}
	transfer.DLNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.DLNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)

	teidBytes := make([]byte, 4)
	teidBytes[0] = byte(teid >> 24)
	teidBytes[1] = byte(teid >> 16)
	teidBytes[2] = byte(teid >> 8)
	teidBytes[3] = byte(teid)
	transfer.DLNGUUPTNLInformation.GTPTunnel.GTPTEID.Value = teidBytes
	transfer.DLNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     ip,
		BitLength: uint64(len(ip) * 8),
	}

	// QosFlowAcceptedList is mandatory (sizeLB:1)
	transfer.QosFlowAcceptedList.List = append(transfer.QosFlowAcceptedList.List,
		ngapType.QosFlowAcceptedItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
		},
	)

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PathSwitchRequestTransfer: %v", err)
	}

	return buf, nil
}

func buildPathSwitchRequest(
	sourceAmfUeNgapID *ngapType.AMFUENGAPID,
	ranUeNgapID *ngapType.RANUENGAPID,
	pduSessionDLList *ngapType.PDUSessionResourceToBeSwitchedDLList,
	failedList *ngapType.PDUSessionResourceFailedToSetupListPSReq,
	uESecurityCapabilities *ngapType.UESecurityCapabilities,
) *ngapType.PathSwitchRequest {
	msg := &ngapType.PathSwitchRequest{}
	ies := &msg.ProtocolIEs

	if ranUeNgapID != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = ranUeNgapID
		ies.List = append(ies.List, ie)
	}

	if sourceAmfUeNgapID != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSourceAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentSourceAMFUENGAPID
		ie.Value.SourceAMFUENGAPID = sourceAmfUeNgapID
		ies.List = append(ies.List, ie)
	}

	if pduSessionDLList != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentPDUSessionResourceToBeSwitchedDLList
		ie.Value.PDUSessionResourceToBeSwitchedDLList = pduSessionDLList
		ies.List = append(ies.List, ie)
	}

	if failedList != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentPDUSessionResourceFailedToSetupListPSReq
		ie.Value.PDUSessionResourceFailedToSetupListPSReq = failedList
		ies.List = append(ies.List, ie)
	}

	if uESecurityCapabilities != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentUESecurityCapabilities
		ie.Value.UESecurityCapabilities = uESecurityCapabilities
		ies.List = append(ies.List, ie)
	}

	return msg
}

func newTestAMFWithSmf(smf amfContext.SmfSbi) *amfContext.AMF {
	return &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc: "001",
				Mnc: "01",
				Sst: 1,
			},
		},
		Radios: map[*sctp.SCTPConn]*amfContext.Radio{},
		Smf:    smf,
	}
}

func newValidAmfUe() *amfContext.AmfUe {
	amfUe := amfContext.NewAmfUe()
	amfUe.Supi = "imsi-001010000000001"
	amfUe.SecurityContextAvailable = true
	amfUe.NgKsi.Ksi = 1
	amfUe.MacFailed = false
	amfUe.Kamf = "0000000000000000000000000000000000000000000000000000000000000000"
	amfUe.NH = make([]byte, 32)
	amfUe.Log = logger.AmfLog

	return amfUe
}

func TestPathSwitchRequest_NilMessage(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
	}
	amf := newTestAMFWithSmf(&FakeSmfSbi{})

	ngap.HandlePathSwitchRequest(context.Background(), amf, ran, nil)

	if len(fakeNGAPSender.SentPathSwitchRequestFailures) != 0 {
		t.Fatalf("expected no PathSwitchRequestFailure for nil message, got %d",
			len(fakeNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_MissingSourceAMFUENGAPID(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}
	amf := newTestAMFWithSmf(&FakeSmfSbi{})

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		nil, // no SourceAMFUENGAPID
		&ngapType.RANUENGAPID{Value: 1},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, ran, msg)

	// Handler should return early without sending anything (silent drop per spec)
	if len(fakeNGAPSender.SentPathSwitchRequestFailures) != 0 {
		t.Fatalf("expected no PathSwitchRequestFailure, got %d",
			len(fakeNGAPSender.SentPathSwitchRequestFailures))
	}

	if len(fakeNGAPSender.SentPathSwitchRequestAcknowledges) != 0 {
		t.Fatalf("expected no PathSwitchRequestAcknowledge, got %d",
			len(fakeNGAPSender.SentPathSwitchRequestAcknowledges))
	}
}

func TestPathSwitchRequest_UnknownUE(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amf := newTestAMFWithSmf(&FakeSmfSbi{})

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 999}, // no UE with this AMF UE NGAP ID
		&ngapType.RANUENGAPID{Value: 1},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(fakeNGAPSender.SentPathSwitchRequestFailures))
	}

	failure := fakeNGAPSender.SentPathSwitchRequestFailures[0]
	if failure.AmfUeNgapID != 999 {
		t.Errorf("expected AmfUeNgapID=999, got %d", failure.AmfUeNgapID)
	}

	if failure.RanUeNgapID != 1 {
		t.Errorf("expected RanUeNgapID=1, got %d", failure.RanUeNgapID)
	}
}

func TestPathSwitchRequest_NilAmfUe(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	// RanUe exists but AmfUe is nil
	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		AmfUe:       nil,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amf := newTestAMFWithSmf(&FakeSmfSbi{})
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 10},
		&ngapType.RANUENGAPID{Value: 2},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	// Should send failure because AmfUe is nil
	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_InvalidSecurityContext(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amfUe := amfContext.NewAmfUe()
	amfUe.SecurityContextAvailable = false
	amfUe.NgKsi.Ksi = nasMessage.NasKeySetIdentifierNoKeyIsAvailable
	amfUe.Log = logger.AmfLog

	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = ranUe
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amf := newTestAMFWithSmf(&FakeSmfSbi{})
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 10},
		&ngapType.RANUENGAPID{Value: 2},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	// Should send failure because security context is not valid
	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_SmContextNotFound(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amfUe := newValidAmfUe()
	// SmContextList is empty — no PDU session ID 1

	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = ranUe
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	fakeSmf := &FakeSmfSbi{PathSwitchResponse: []byte{0x01}}
	amf := newTestAMFWithSmf(fakeSmf)
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 10},
		&ngapType.RANUENGAPID{Value: 2},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	// No PDU sessions could be switched; should send failure
	if len(fakeSmf.PathSwitchCalls) != 0 {
		t.Fatalf("expected no SMF calls, got %d", len(fakeSmf.PathSwitchCalls))
	}

	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_SmfReturnsError(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.SmContextList[1] = &amfContext.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = ranUe
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchErr: fmt.Errorf("PFCP modification failed"),
	}
	amf := newTestAMFWithSmf(fakeSmf)
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 10},
		&ngapType.RANUENGAPID{Value: 2},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	// SMF call should have been made
	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 SMF PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	if fakeSmf.PathSwitchCalls[0].SmContextRef != "imsi-001010000000001-1" {
		t.Errorf("expected SmContextRef=imsi-001010000000001-1, got %s", fakeSmf.PathSwitchCalls[0].SmContextRef)
	}

	// All PDU sessions failed -> should send PathSwitchRequestFailure
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

	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.Kamf = kamfHex
	amfUe.SmContextList[pduSessionID] = &amfContext.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: sourceAmfUeNgapID,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = sourceUe
	sourceRan.RanUEs[1] = sourceUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	n2Response := []byte{0xAA, 0xBB, 0xCC}
	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: n2Response,
	}

	amf := newTestAMFWithSmf(fakeSmf)
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: sourceAmfUeNgapID},
		&ngapType.RANUENGAPID{Value: targetRanUeNgapID},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: int64(pduSessionID)},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	// Verify SMF was called
	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 SMF PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	if fakeSmf.PathSwitchCalls[0].SmContextRef != "imsi-001010000000001-1" {
		t.Errorf("expected SmContextRef=imsi-001010000000001-1, got %s", fakeSmf.PathSwitchCalls[0].SmContextRef)
	}

	// Verify PathSwitchRequestAcknowledge was sent
	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]

	if ack.AmfUeNgapID != sourceAmfUeNgapID {
		t.Errorf("expected AmfUeNgapID=%d, got %d", sourceAmfUeNgapID, ack.AmfUeNgapID)
	}

	if ack.RanUeNgapID != targetRanUeNgapID {
		t.Errorf("expected RanUeNgapID=%d, got %d", targetRanUeNgapID, ack.RanUeNgapID)
	}

	if ack.NCC == 0 {
		t.Error("expected NCC > 0 after UpdateNH")
	}

	// Verify the PDU session switched list contains the session
	if len(ack.PDUSessionResourceSwitchedList.List) != 1 {
		t.Fatalf("expected 1 switched PDU session, got %d", len(ack.PDUSessionResourceSwitchedList.List))
	}

	if ack.PDUSessionResourceSwitchedList.List[0].PDUSessionID.Value != int64(pduSessionID) {
		t.Errorf("expected PDU session ID %d, got %d", pduSessionID, ack.PDUSessionResourceSwitchedList.List[0].PDUSessionID.Value)
	}

	// Verify the RAN UE was switched to the new RAN
	if sourceUe.Radio != targetRan {
		t.Error("expected RanUe to be switched to targetRan")
	}

	if sourceUe.RanUeNgapID != targetRanUeNgapID {
		t.Errorf("expected RanUeNgapID=%d, got %d", targetRanUeNgapID, sourceUe.RanUeNgapID)
	}

	// No failures should be sent
	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 0 {
		t.Fatalf("expected no PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_MultiplePDUSessions_PartialSuccess(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amfUe := newValidAmfUe()
	// Session 1 has a context, session 2 does not
	amfUe.SmContextList[1] = &amfContext.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = ranUe
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amf := newTestAMFWithSmf(fakeSmf)
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer1, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer1: %v", err)
	}

	transfer2, err := buildPathSwitchRequestTransfer(6000, []byte{10, 0, 0, 3})
	if err != nil {
		t.Fatalf("failed to build transfer2: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 10},
		&ngapType.RANUENGAPID{Value: 2},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer1,
				},
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 2}, // No SmContext for this
					PathSwitchRequestTransfer: transfer2,
				},
			},
		},
		nil, nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	// Only session 1 should succeed
	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 SMF PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	// Should still send acknowledge (at least one session succeeded)
	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]
	if len(ack.PDUSessionResourceSwitchedList.List) != 1 {
		t.Fatalf("expected 1 switched PDU session, got %d",
			len(ack.PDUSessionResourceSwitchedList.List))
	}

	if ack.PDUSessionResourceSwitchedList.List[0].PDUSessionID.Value != 1 {
		t.Errorf("expected PDU session 1 to be switched, got %d",
			ack.PDUSessionResourceSwitchedList.List[0].PDUSessionID.Value)
	}
}

func TestPathSwitchRequest_FailedPDUSessionsReportedToSmf(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.SmContextList[1] = &amfContext.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}
	amfUe.SmContextList[2] = &amfContext.SmContext{
		Ref:    "imsi-001010000000001-2",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = ranUe
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amf := newTestAMFWithSmf(fakeSmf)
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	// Build a failed transfer
	failedTransfer := ngapType.PathSwitchRequestSetupFailedTransfer{
		Cause: ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		},
	}

	failedBytes, err := aper.MarshalWithParams(failedTransfer, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal failed transfer: %v", err)
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 10},
		&ngapType.RANUENGAPID{Value: 2},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		&ngapType.PDUSessionResourceFailedToSetupListPSReq{
			List: []ngapType.PDUSessionResourceFailedToSetupItemPSReq{
				{
					PDUSessionID:                         ngapType.PDUSessionID{Value: 2},
					PathSwitchRequestSetupFailedTransfer: failedBytes,
				},
			},
		},
		nil,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	// Path switch succeeded for session 1
	if len(fakeSmf.PathSwitchCalls) != 1 {
		t.Fatalf("expected 1 PathSwitch call, got %d", len(fakeSmf.PathSwitchCalls))
	}

	// Handover failed reported for session 2
	if len(fakeSmf.HandoverFailedCalls) != 1 {
		t.Fatalf("expected 1 HandoverFailed call, got %d", len(fakeSmf.HandoverFailedCalls))
	}

	if fakeSmf.HandoverFailedCalls[0].SmContextRef != "imsi-001010000000001-2" {
		t.Errorf("expected SmContextRef=imsi-001010000000001-2, got %s", fakeSmf.HandoverFailedCalls[0].SmContextRef)
	}

	// Should send acknowledge
	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}
}

func TestPathSwitchRequest_UESecurityCapabilitiesUpdated(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.UESecurityCapability = &nasType.UESecurityCapability{}
	amfUe.UESecurityCapability.SetLen(4) // allocate Buffer for 5G + E-UTRA algorithms
	amfUe.SmContextList[1] = &amfContext.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = ranUe
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amfContext.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amf := newTestAMFWithSmf(fakeSmf)
	amf.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	// Set UE security capabilities with EA1 and IA2
	secCap := &ngapType.UESecurityCapabilities{}
	secCap.NRencryptionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x80, 0x00}, // EA1 set
		BitLength: 16,
	}
	secCap.NRintegrityProtectionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x40, 0x00}, // IA2 set
		BitLength: 16,
	}
	secCap.EUTRAencryptionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x00, 0x00},
		BitLength: 16,
	}
	secCap.EUTRAintegrityProtectionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x00, 0x00},
		BitLength: 16,
	}

	msg := buildPathSwitchRequest(
		&ngapType.AMFUENGAPID{Value: 10},
		&ngapType.RANUENGAPID{Value: 2},
		&ngapType.PDUSessionResourceToBeSwitchedDLList{
			List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
				{
					PDUSessionID:              ngapType.PDUSessionID{Value: 1},
					PathSwitchRequestTransfer: transfer,
				},
			},
		},
		nil,
		secCap,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amf, targetRan, msg)

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}
}
