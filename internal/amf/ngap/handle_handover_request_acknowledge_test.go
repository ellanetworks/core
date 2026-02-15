// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"net"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
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
func setupHandoverAckTestContext(t *testing.T) (*amfContext.Radio, *FakeNGAPSender, *amfContext.AMF) {
	t.Helper()

	const (
		pduSessionID = uint8(1)
		supi         = "imsi-001010000000001"
		dnn          = "internet"
	)

	smfContext.InitializeSMF(nil)

	smf := smfContext.SMFSelf()
	smCtx := smf.NewSMContext(supi, pduSessionID, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &models.SmPolicyData{
		Ambr: &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		QosData: &models.QosData{
			QFI:    1,
			Var5qi: 9,
			Arp:    &models.Arp{PriorityLevel: 8},
		},
	}
	smCtx.Tunnel = &smfContext.UPTunnel{
		DataPath: &smfContext.DataPath{
			UpLinkTunnel: &smfContext.GTPTunnel{
				TEID: 1234,
				N3IP: net.ParseIP("10.0.0.1").To4(),
			},
		},
	}

	amfUe := amfContext.NewAmfUe()
	amfUe.Supi = supi
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[pduSessionID] = &amfContext.SmContext{
		Ref:    smfContext.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	sourceUe := &amfContext.RanUe{
		RanUeNgapID: 10,
		AmfUeNgapID: 100,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = sourceUe
	sourceRan.RanUEs[10] = sourceUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    targetNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	targetUe := &amfContext.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		AmfUe:       amfUe,
		SourceUe:    sourceUe,
		Radio:       targetRan,
		Log:         logger.AmfLog,
	}
	sourceUe.TargetUe = targetUe
	targetRan.RanUEs[2] = targetUe

	amf := &amfContext.AMF{
		Radios: map[*sctp.SCTPConn]*amfContext.Radio{
			new(sctp.SCTPConn): sourceRan,
			new(sctp.SCTPConn): targetRan,
		},
	}

	return targetRan, sourceNGAPSender, amf
}

func TestHandoverRequestAcknowledge_NilMessage(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
	}
	amf := &amfContext.AMF{}

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amf, ran, nil)

	if len(fakeNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}

	if len(fakeNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(fakeNGAPSender.SentHandoverCommands))
	}
}

func TestHandoverRequestAcknowledge_MissingTargetToSourceContainer(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
	}
	amf := &amfContext.AMF{}

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
		// TargetToSourceTransparentContainer intentionally omitted
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amf, ran, msg)

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
	ran := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	amf := &amfContext.AMF{
		Radios: map[*sctp.SCTPConn]*amfContext.Radio{
			new(sctp.SCTPConn): ran,
		},
	}

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 999},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(fakeNGAPSender.SentHandoverCommands))
	}

	if len(fakeNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}
}

func TestHandoverRequestAcknowledge_NoSourceUe(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	amfUe := amfContext.NewAmfUe()
	amfUe.Log = logger.AmfLog

	targetUe := &amfContext.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		AmfUe:       amfUe,
		SourceUe:    nil,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[2] = targetUe

	amf := &amfContext.AMF{
		Radios: map[*sctp.SCTPConn]*amfContext.Radio{
			new(sctp.SCTPConn): ran,
		},
	}

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
		TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(fakeNGAPSender.SentHandoverCommands))
	}

	if len(fakeNGAPSender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(fakeNGAPSender.SentHandoverPreparationFailures))
	}
}

func TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SendsPreparationFailure(t *testing.T) {
	targetRan, sourceNGAPSender, amf := setupHandoverAckTestContext(t)

	msg := buildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
		TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amf, targetRan, msg)

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
	targetRan, sourceNGAPSender, amf := setupHandoverAckTestContext(t)

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

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amf, targetRan, msg)

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
