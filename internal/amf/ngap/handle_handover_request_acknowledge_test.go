// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// setupHandoverAckTestContext creates the AMF, source/target UEs, radios, and
// SMF context needed for handover request acknowledge tests.
func setupHandoverAckTestContext(t *testing.T) (*amf.Radio, *fakeNGAPSender, *amf.AMF) {
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
				TEID:   1234,
				N3IPv4: netip.MustParseAddr("10.0.0.1"),
			},
		},
	}

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smf.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:           logger.AmfLog,
		Conn:          sourceNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	sourceRan.BindAMFForTest(amf.New(nil, nil, nil))

	sourceUe := amf.NewRanUeForTest(sourceRan, 10, 100, logger.AmfLog)
	amfUe.AttachRanUe(sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:           logger.AmfLog,
		Conn:          targetNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	targetRan.BindAMFForTest(amf.New(nil, nil, nil))

	targetUe := amf.NewRanUeForTest(targetRan, 2, 1, logger.AmfLog)

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	amfInstance := amf.New(nil, nil, &fakeSmfSbi{SMF: smfInstance})
	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan
	amfInstance.Radios[new(sctp.SCTPConn)] = targetRan

	return targetRan, sourceNGAPSender, amfInstance
}

func TestHandoverRequestAcknowledge_UeNotFound(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		Conn:          sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

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

	if len(sender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(sender.SentHandoverCommands))
	}

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication (TS 38.413), got %d", len(sender.SentErrorIndications))
	}
}

func TestHandoverRequestAcknowledge_NoSourceUe(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		Conn:          sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	targetUe := amf.NewRanUeForTest(ran, 2, 1, logger.AmfLog)
	amfUe.AttachRanUe(targetUe)
	// No handover installed, so HandoverSource() is nil.

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

	if len(sender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(sender.SentHandoverCommands))
	}

	if len(sender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(sender.SentHandoverPreparationFailures))
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

// TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SourceUeContextDetached
// verifies that when no PDU sessions are admitted and the source UE's AMF UE
// context has been detached (e.g. due to a concurrent deregistration), the
// handler does not panic.
func TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SourceUeContextDetached(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	targetUe := targetRan.FindUEByAmfUeNgapID(1)
	if targetUe == nil {
		t.Fatal("target UE not found on target radio")
	}

	sourceUeContext := targetUe.UeContext()
	if sourceUeContext == nil {
		t.Fatal("source AMF UE not found")
	}

	sourceUeContext.ReleaseNasConnection(nil)

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

// TestHandoverRequestAcknowledge_PartialAdmission verifies that when the target
// admits some PDU sessions and fails others, the Handover Command confirms the
// admitted ones in the Handover List and lists the failed ones in the PDU
// Session Resource To Release List (TS 38.413).
func TestHandoverRequestAcknowledge_PartialAdmission(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	admittedTransfer := ngapType.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: ngapType.UPTransportLayerInformation{
			Present: ngapType.UPTransportLayerInformationPresentGTPTunnel,
			GTPTunnel: &ngapType.GTPTunnel{
				TransportLayerAddress: ngapType.TransportLayerAddress{
					Value: aper.BitString{Bytes: []byte{10, 0, 0, 2}, BitLength: 32},
				},
				GTPTEID: ngapType.GTPTEID{Value: []byte{0x00, 0x00, 0x04, 0xD2}},
			},
		},
		QosFlowSetupResponseList: ngapType.QosFlowListWithDataForwarding{
			List: []ngapType.QosFlowItemWithDataForwarding{
				{QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1}},
			},
		},
	}

	admittedBytes, err := aper.MarshalWithParams(admittedTransfer, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal admitted transfer: %v", err)
	}

	unsuccessfulTransfer := ngapType.HandoverResourceAllocationUnsuccessfulTransfer{
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable},
		},
	}

	unsuccessfulBytes, err := aper.MarshalWithParams(unsuccessfulTransfer, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal unsuccessful transfer: %v", err)
	}

	amfID := int64(1)
	ranID := int64(2)
	msg := decode.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		AdmittedItems: []ngapType.PDUSessionResourceAdmittedItem{
			{PDUSessionID: ngapType.PDUSessionID{Value: 1}, HandoverRequestAcknowledgeTransfer: admittedBytes},
		},
		FailedToSetupItems: []ngapType.PDUSessionResourceFailedToSetupItemHOAck{
			{PDUSessionID: ngapType.PDUSessionID{Value: 2}, HandoverResourceAllocationUnsuccessfulTransfer: unsuccessfulBytes},
		},
		TargetToSourceTransparentContainer: ngapType.TargetToSourceTransparentContainer{Value: []byte{0xAA, 0xBB, 0xCC}},
	}

	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentHandoverCommands) != 1 {
		t.Fatalf("expected 1 HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}

	cmd := sourceNGAPSender.SentHandoverCommands[0]

	if len(cmd.HandoverList.List) != 1 || cmd.HandoverList.List[0].PDUSessionID.Value != 1 {
		t.Errorf("expected handover list to confirm session 1, got %+v", cmd.HandoverList.List)
	}

	if len(cmd.ToReleaseList.List) != 1 || cmd.ToReleaseList.List[0].PDUSessionID.Value != 2 {
		t.Fatalf("expected to-release list to contain session 2 (TS 38.413), got %+v", cmd.ToReleaseList.List)
	}

	// The to-release item must carry a decodable HandoverPreparationUnsuccessfulTransfer.
	var relayed ngapType.HandoverPreparationUnsuccessfulTransfer
	if err := aper.UnmarshalWithParams(cmd.ToReleaseList.List[0].HandoverPreparationUnsuccessfulTransfer, &relayed, "valueExt"); err != nil {
		t.Fatalf("to-release transfer does not decode: %v", err)
	}

	if relayed.Cause.Present != ngapType.CausePresentRadioNetwork ||
		relayed.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable {
		t.Errorf("expected relayed cause RadioResourcesNotAvailable, got %+v", relayed.Cause)
	}
}
