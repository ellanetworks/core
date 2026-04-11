// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

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

	smfInstance := smf.New(nil, nil, nil, nil)

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
				N3IP: netip.MustParseAddr("10.0.0.1"),
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

	amfID := int64(999)
	ranID := int64(1)
	msg := decode.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		TargetToSourceTransparentContainer: ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	}

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

	amfID := int64(1)
	ranID := int64(2)
	msg := decode.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		TargetToSourceTransparentContainer: ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	}

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

	amfID := int64(1)
	ranID := int64(2)
	msg := decode.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		TargetToSourceTransparentContainer: ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	}

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

// TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SourceAmfUeDetached
// verifies that when no PDU sessions are admitted and the source UE's AMF UE
// context has been detached (e.g. due to a concurrent deregistration), the
// handler does not panic.
func TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SourceAmfUeDetached(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	targetUe := amfInstance.FindRanUeByAmfUeNgapID(1)
	if targetUe == nil {
		t.Fatal("target UE not found")
	}

	sourceAmfUe := targetUe.SourceUe.AmfUe()
	if sourceAmfUe == nil {
		t.Fatal("source AMF UE not found")
	}

	sourceAmfUe.DetachRanUe(nil)

	amfID := int64(1)
	ranID := int64(2)
	msg := decode.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		TargetToSourceTransparentContainer: ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	}

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure on source radio, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
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

	amfID := int64(1)
	ranID := int64(2)
	msg := decode.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		AdmittedItems: []ngapType.PDUSessionResourceAdmittedItem{
			{
				PDUSessionID:                       ngapType.PDUSessionID{Value: 1},
				HandoverRequestAcknowledgeTransfer: transferBytes,
			},
		},
		TargetToSourceTransparentContainer: ngapType.TargetToSourceTransparentContainer{
			Value: containerData,
		},
	}

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
