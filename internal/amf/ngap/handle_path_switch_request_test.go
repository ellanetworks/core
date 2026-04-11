// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
)

// decodePathSwitchRequestOrFatal decodes msg and fails the test only if
// the decoder reports a fatal error. Non-fatal reports (e.g. missing
// mandatory-ignore IEs like UserLocationInformation that the legacy
// builder omits) are accepted: the dispatcher would invoke the handler
// in that case anyway.
func decodePathSwitchRequestOrFatal(t *testing.T, msg *ngapType.PathSwitchRequest) decode.PathSwitchRequest {
	t.Helper()

	decoded, report := decode.DecodePathSwitchRequest(msg)
	if report != nil && report.Fatal() {
		t.Fatalf("decoder produced fatal report: %+v", report)
	}

	return decoded
}

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

func newTestAMFWithSmf(smf amf.SmfSbi) *amf.AMF {
	return amf.New(&FakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, smf)
}

func newValidAmfUe() *amf.AmfUe {
	amfUe := amf.NewAmfUe()
	amfUe.Supi, _ = etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	amfUe.SecurityContextAvailable = true
	amfUe.NgKsi.Ksi = 1
	amfUe.MacFailed = false
	amfUe.Kamf = "0000000000000000000000000000000000000000000000000000000000000000"
	amfUe.NH = make([]byte, 32)
	amfUe.Log = logger.AmfLog

	return amfUe
}

func TestPathSwitchRequest_UnknownUE(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, ran, decodePathSwitchRequestOrFatal(t, msg))

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
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	// RanUe exists but AmfUe is nil
	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

	// Should send failure because AmfUe is nil
	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_InvalidSecurityContext(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := amf.NewAmfUe()
	amfUe.SecurityContextAvailable = false
	amfUe.NgKsi.Ksi = nasMessage.NasKeySetIdentifierNoKeyIsAvailable
	amfUe.Log = logger.AmfLog

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

	// Should send failure because security context is not valid
	if len(targetNGAPSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestFailure, got %d",
			len(targetNGAPSender.SentPathSwitchRequestFailures))
	}
}

func TestPathSwitchRequest_SmContextNotFound(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	// SmContextList is empty — no PDU session ID 1

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	fakeSmf := &FakeSmfSbi{PathSwitchResponse: []byte{0x01}}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

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
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchErr: fmt.Errorf("PFCP modification failed"),
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

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
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.Kamf = kamfHex
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: sourceAmfUeNgapID,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(sourceUe)
	sourceRan.RanUEs[1] = sourceUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	n2Response := []byte{0xAA, 0xBB, 0xCC}
	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: n2Response,
	}

	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

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
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	// Session 1 has a context, session 2 does not
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

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
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}
	amfUe.SmContextList[2] = &amf.SmContext{
		Ref:    "imsi-001010000000001-2",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

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

// TestPathSwitchRequest_UESecurityCapabilitiesNotOverwritten verifies the
// AMF keeps its stored UE 5G security capabilities when the target gNB
// reports different values in a PathSwitchRequest (TS 33.501 §6.7.3.1).
func TestPathSwitchRequest_UESecurityCapabilitiesNotOverwritten(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.UESecurityCapability = &nasType.UESecurityCapability{}
	amfUe.UESecurityCapability.SetLen(4)
	amfUe.UESecurityCapability.SetEA1_128_5G(1)
	amfUe.UESecurityCapability.SetEA2_128_5G(1)
	amfUe.UESecurityCapability.SetEA3_128_5G(1)
	amfUe.UESecurityCapability.SetIA1_128_5G(1)
	amfUe.UESecurityCapability.SetIA2_128_5G(1)
	amfUe.UESecurityCapability.SetIA3_128_5G(1)
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	secCap := &ngapType.UESecurityCapabilities{}
	secCap.NRencryptionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x00, 0x00},
		BitLength: 16,
	}
	secCap.NRintegrityProtectionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x00, 0x00},
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

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

	if got := amfUe.UESecurityCapability.GetEA1_128_5G(); got != 1 {
		t.Errorf("stored EA1_128_5G was overwritten: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetEA2_128_5G(); got != 1 {
		t.Errorf("stored EA2_128_5G was overwritten: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetEA3_128_5G(); got != 1 {
		t.Errorf("stored EA3_128_5G was overwritten: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetIA1_128_5G(); got != 1 {
		t.Errorf("stored IA1_128_5G was overwritten: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetIA2_128_5G(); got != 1 {
		t.Errorf("stored IA2_128_5G was overwritten: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetIA3_128_5G(); got != 1 {
		t.Errorf("stored IA3_128_5G was overwritten: got %d, want 1", got)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]
	if ack.UESecurityCapability == nil {
		t.Fatal("PathSwitchRequestAcknowledge has nil UESecurityCapability")
	}

	if ack.UESecurityCapability.GetEA1_128_5G() != 1 ||
		ack.UESecurityCapability.GetEA2_128_5G() != 1 ||
		ack.UESecurityCapability.GetEA3_128_5G() != 1 ||
		ack.UESecurityCapability.GetIA1_128_5G() != 1 ||
		ack.UESecurityCapability.GetIA2_128_5G() != 1 ||
		ack.UESecurityCapability.GetIA3_128_5G() != 1 {
		t.Error("PathSwitchRequestAcknowledge does not echo locally stored UE security capabilities")
	}
}

// TestPathSwitchRequest_UESecurityCapabilitiesMatching exercises the
// happy path where the target gNB reports the same capabilities the AMF
// has stored.
func TestPathSwitchRequest_UESecurityCapabilitiesMatching(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.UESecurityCapability = &nasType.UESecurityCapability{}
	amfUe.UESecurityCapability.SetLen(4)
	amfUe.UESecurityCapability.SetEA1_128_5G(1)
	amfUe.UESecurityCapability.SetIA2_128_5G(1)
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	matchingCaps := &ngapType.UESecurityCapabilities{}
	matchingCaps.NRencryptionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x80, 0x00}, // EA1
		BitLength: 16,
	}
	matchingCaps.NRintegrityProtectionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x40, 0x00}, // IA2
		BitLength: 16,
	}
	matchingCaps.EUTRAencryptionAlgorithms.Value = aper.BitString{
		Bytes:     []byte{0x00, 0x00},
		BitLength: 16,
	}
	matchingCaps.EUTRAintegrityProtectionAlgorithms.Value = aper.BitString{
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
		matchingCaps,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

	if got := amfUe.UESecurityCapability.GetEA1_128_5G(); got != 1 {
		t.Errorf("stored EA1_128_5G changed after matching path switch: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetIA2_128_5G(); got != 1 {
		t.Errorf("stored IA2_128_5G changed after matching path switch: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetEA2_128_5G(); got != 0 {
		t.Errorf("stored EA2_128_5G unexpectedly set: got %d, want 0", got)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetNGAPSender.SentPathSwitchRequestAcknowledges[0]
	if ack.UESecurityCapability == nil {
		t.Fatal("PathSwitchRequestAcknowledge has nil UESecurityCapability")
	}

	if ack.UESecurityCapability.GetEA1_128_5G() != 1 || ack.UESecurityCapability.GetIA2_128_5G() != 1 {
		t.Error("PathSwitchRequestAcknowledge does not echo locally stored UE security capabilities")
	}
}

// TestPathSwitchRequest_EmptySecurityCapabilityBytes covers a
// PathSwitchRequest whose UESecurityCapabilities IE has empty NR
// bitstrings: the handler must not panic, must leave stored capabilities
// untouched, and must still emit the PathSwitchRequestAcknowledge.
func TestPathSwitchRequest_EmptySecurityCapabilityBytes(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: sourceNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	amfUe := newValidAmfUe()
	amfUe.UESecurityCapability = &nasType.UESecurityCapability{}
	amfUe.UESecurityCapability.SetLen(4)
	amfUe.UESecurityCapability.SetEA1_128_5G(1)
	amfUe.UESecurityCapability.SetIA2_128_5G(1)
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	sourceRan.RanUEs[1] = ranUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanUEs:     make(map[int64]*amf.RanUe),
	}

	fakeSmf := &FakeSmfSbi{
		PathSwitchResponse: []byte{0xAA},
	}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("failed to build transfer: %v", err)
	}

	emptyCaps := &ngapType.UESecurityCapabilities{
		NRencryptionAlgorithms: ngapType.NRencryptionAlgorithms{
			Value: aper.BitString{Bytes: []byte{}, BitLength: 0},
		},
		NRintegrityProtectionAlgorithms: ngapType.NRintegrityProtectionAlgorithms{
			Value: aper.BitString{Bytes: []byte{}, BitLength: 0},
		},
		EUTRAencryptionAlgorithms: ngapType.EUTRAencryptionAlgorithms{
			Value: aper.BitString{Bytes: []byte{}, BitLength: 0},
		},
		EUTRAintegrityProtectionAlgorithms: ngapType.EUTRAintegrityProtectionAlgorithms{
			Value: aper.BitString{Bytes: []byte{}, BitLength: 0},
		},
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
		emptyCaps,
	)

	ngap.HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, decodePathSwitchRequestOrFatal(t, msg))

	if got := amfUe.UESecurityCapability.GetEA1_128_5G(); got != 1 {
		t.Errorf("stored EA1_128_5G was modified by malformed IE: got %d, want 1", got)
	}

	if got := amfUe.UESecurityCapability.GetIA2_128_5G(); got != 1 {
		t.Errorf("stored IA2_128_5G was modified by malformed IE: got %d, want 1", got)
	}

	if len(targetNGAPSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge despite malformed security IE, got %d",
			len(targetNGAPSender.SentPathSwitchRequestAcknowledges))
	}
}
