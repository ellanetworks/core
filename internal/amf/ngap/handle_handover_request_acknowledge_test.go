// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"net"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type HandoverRequestAcknowledgeOpts struct {
	AMFUENGAPID                              *ngapType.AMFUENGAPID
	RANUENGAPID                              *ngapType.RANUENGAPID
	PDUSessionResourceAdmittedList           *ngapType.PDUSessionResourceAdmittedList
	PDUSessionResourceFailedToSetupListHOAck *ngapType.PDUSessionResourceFailedToSetupListHOAck
	TargetToSourceTransparentContainer       *ngapType.TargetToSourceTransparentContainer
}

// buildHandoverRequestAcknowledge constructs an NGAP HandoverRequestAcknowledge
// message from the given options.
func buildHandoverRequestAcknowledge(opts *HandoverRequestAcknowledgeOpts) *ngapType.HandoverRequestAcknowledge {
	if opts == nil {
		return nil
	}

	msg := &ngapType.HandoverRequestAcknowledge{}
	ies := &msg.ProtocolIEs

	if opts.AMFUENGAPID != nil {
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentAMFUENGAPID
		ie.Value.AMFUENGAPID = opts.AMFUENGAPID
		ies.List = append(ies.List, ie)
	}

	if opts.RANUENGAPID != nil {
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = opts.RANUENGAPID
		ies.List = append(ies.List, ie)
	}

	if opts.PDUSessionResourceAdmittedList != nil {
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceAdmittedList
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentPDUSessionResourceAdmittedList
		ie.Value.PDUSessionResourceAdmittedList = opts.PDUSessionResourceAdmittedList
		ies.List = append(ies.List, ie)
	}

	if opts.PDUSessionResourceFailedToSetupListHOAck != nil {
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentPDUSessionResourceFailedToSetupListHOAck
		ie.Value.PDUSessionResourceFailedToSetupListHOAck = opts.PDUSessionResourceFailedToSetupListHOAck
		ies.List = append(ies.List, ie)
	}

	if opts.TargetToSourceTransparentContainer != nil {
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDTargetToSourceTransparentContainer
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentTargetToSourceTransparentContainer
		ie.Value.TargetToSourceTransparentContainer = opts.TargetToSourceTransparentContainer
		ies.List = append(ies.List, ie)
	}

	return msg
}

// setupHandoverAckTestContext creates the AMF, source/target UEs, radios, and
// SMF context needed for handover request acknowledge tests.
func setupHandoverAckTestContext(t *testing.T) (*amf.Radio, *FakeNGAPSender, *amf.AMF) {
	t.Helper()

	const (
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	smfInstance := smf.New(nil, nil, nil)

	smCtx := smfInstance.NewSession(supi, pduSessionID, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr: models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		QosData: models.QosData{
			QFI:    1,
			Var5qi: 9,
			Arp:    &models.Arp{PriorityLevel: 8},
		},
	}
	smCtx.Tunnel = &smf.UPTunnel{
		DataPath: &smf.DataPath{
			UpLinkTunnel: &smf.GTPTunnel{
				TEID: 1234,
				N3IP: net.ParseIP("10.0.0.1").To4(),
			},
		},
	}

	amfUe := amf.NewAmfUe()
	amfUe.Supi = supi
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smf.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	sourceUe := &amf.RanUe{
		RanUeNgapID: 10,
		AmfUeNgapID: 100,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(sourceUe)
	sourceRan.RanUEs[10] = sourceUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    targetNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		Radio:       targetRan,
		Log:         logger.AmfLog,
	}

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	targetRan.RanUEs[2] = targetUe

	amfInstance := amf.New(nil, nil, &FakeSmfSbi{SMF: smfInstance})
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan
	amfInstance.Radios[new(sctp.SCTPConn)] = targetRan

	return targetRan, sourceNGAPSender, amfInstance
}

func TestHandoverRequestAcknowledge_NilMessage(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
	}
	amfInstance := newTestAMF()

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, ran, nil)

	if len(fakeNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}

	if len(fakeNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(fakeNGAPSender.SentHandoverCommands))
	}
}

func TestHandoverRequestAcknowledge_MissingTargetToSourceContainer(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
	}
	amfInstance := newTestAMF()

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
		// TargetToSourceTransparentContainer intentionally omitted
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, ran, msg)

	if len(fakeNGAPSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}

	errorIndication := fakeNGAPSender.SentErrorIndications[0]
	if errorIndication.CriticalityDiagnostics == nil {
		t.Fatal("expected CriticalityDiagnostics in ErrorIndication, got nil")
	}

	ieList := errorIndication.CriticalityDiagnostics.IEsCriticalityDiagnostics
	if ieList == nil || len(ieList.List) != 1 {
		count := 0
		if ieList != nil {
			count = len(ieList.List)
		}

		t.Fatalf("expected 1 missing IE diagnostic, got %d", count)
	}

	if ieList.List[0].IEID.Value != ngapType.ProtocolIEIDTargetToSourceTransparentContainer {
		t.Fatalf("expected missing IE to be TargetToSourceTransparentContainer, got %d", ieList.List[0].IEID.Value)
	}

	if len(fakeNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand after missing IE, got %d", len(fakeNGAPSender.SentHandoverCommands))
	}
}

func TestHandoverRequestAcknowledge_UeNotFound(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	amfInstance := newTestAMF()
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 999},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, ran, msg)

	if len(fakeNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(fakeNGAPSender.SentHandoverCommands))
	}

	if len(fakeNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}
}

func TestHandoverRequestAcknowledge_NoSourceUe(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		SourceUe:    nil,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(targetUe)
	ran.RanUEs[2] = targetUe

	amfInstance := newTestAMF()
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
		TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, ran, msg)

	if len(fakeNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(fakeNGAPSender.SentHandoverCommands))
	}

	if len(fakeNGAPSender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(fakeNGAPSender.SentHandoverPreparationFailures))
	}
}

func TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SendsPreparationFailure(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
		TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	failure := sourceNGAPSender.SentHandoverPreparationFailures[0]

	if failure.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got present=%d", failure.Cause.Present)
	}

	if failure.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem {
		t.Fatalf("expected HoFailureInTarget5GCNgranNodeOrTargetSystem, got %d", failure.Cause.RadioNetwork.Value)
	}

	if len(sourceNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}
}

func TestHandoverRequestAcknowledge_HappyPath(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	hoAckTransfer := ngapType.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: ngapType.UPTransportLayerInformation{
			Present: ngapType.UPTransportLayerInformationPresentGTPTunnel,
			GTPTunnel: &ngapType.GTPTunnel{
				TransportLayerAddress: ngapType.TransportLayerAddress{
					Value: aper.BitString{
						Bytes:     []byte{10, 0, 0, 2},
						BitLength: 32,
					},
				},
				GTPTEID: ngapType.GTPTEID{
					Value: []byte{0x00, 0x00, 0x04, 0xD2},
				},
			},
		},
		QosFlowSetupResponseList: ngapType.QosFlowListWithDataForwarding{
			List: []ngapType.QosFlowItemWithDataForwarding{
				{
					QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
				},
			},
		},
	}

	transferBytes, err := aper.MarshalWithParams(hoAckTransfer, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequestAcknowledgeTransfer: %v", err)
	}

	containerData := []byte{0xAA, 0xBB, 0xCC}
	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
		PDUSessionResourceAdmittedList: &ngapType.PDUSessionResourceAdmittedList{
			List: []ngapType.PDUSessionResourceAdmittedItem{
				{
					PDUSessionID:                       ngapType.PDUSessionID{Value: 1},
					HandoverRequestAcknowledgeTransfer: transferBytes,
				},
			},
		},
		TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{
			Value: containerData,
		},
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentHandoverCommands) != 1 {
		t.Fatalf("expected 1 HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}

	cmd := sourceNGAPSender.SentHandoverCommands[0]
	if cmd.AmfUeNgapID != 100 {
		t.Errorf("expected AmfUeNgapID=100, got %d", cmd.AmfUeNgapID)
	}

	if cmd.RanUeNgapID != 10 {
		t.Errorf("expected RanUeNgapID=10 (source), got %d", cmd.RanUeNgapID)
	}

	if len(cmd.Container.Value) != len(containerData) {
		t.Errorf("expected container length %d, got %d", len(containerData), len(cmd.Container.Value))
	}

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}
}
