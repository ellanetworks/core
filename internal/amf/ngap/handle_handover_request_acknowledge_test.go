// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/ngap"
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

	smCtx := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr: models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")},
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
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smf.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}

	amfInstance := amf.New(nil, nil, &fakeSmfSbi{SMF: smfInstance})
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 1, logger.AmfLog)

	err := amf.SetHandoverForTest(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	return targetRan, sourceNGAPSender, amfInstance
}

func TestHandoverRequestAcknowledge_UeNotFound(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	amfInstance := newTestAMF()
	ran.BindAMFForTest(amfInstance)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfID := ngap.AMFUENGAPID(999)
	ranID := ngap.RANUENGAPID(1)
	msg := ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        &amfID,
		RANUENGAPID:                        &ranID,
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0x01, 0x02, 0x03},
	}

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, ran, &msg)

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
		Log:  logger.AmfLog,
		Conn: sender,
	}
	amfInstance := newTestAMF()
	ran.BindAMFForTest(amfInstance)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfUe := amf.NewUeContext()

	targetUe := amf.NewUeConnForTest(ran, 2, 1, logger.AmfLog)
	targetUe.AMFForTest().AttachUeConn(amfUe, targetUe)
	// No handover installed, so HandoverSource() is nil.

	amfID := ngap.AMFUENGAPID(1)
	ranID := ngap.RANUENGAPID(2)
	msg := ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        &amfID,
		RANUENGAPID:                        &ranID,
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0x01, 0x02, 0x03},
	}

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, ran, &msg)

	if len(sender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(sender.SentHandoverCommands))
	}

	if len(sender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(sender.SentHandoverPreparationFailures))
	}
}

func TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SendsPreparationFailure(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	amfID := ngap.AMFUENGAPID(1)
	ranID := ngap.RANUENGAPID(2)
	msg := ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        &amfID,
		RANUENGAPID:                        &ranID,
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0x01, 0x02, 0x03},
	}

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, &msg)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	failure := sourceNGAPSender.SentHandoverPreparationFailures[0]

	wantRadioNetworkCause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget}
	if failure.Cause == nil || *failure.Cause != wantRadioNetworkCause {
		t.Errorf("cause = %v, want ho-failure-in-target-5GC-ngran-node-or-target-system", failure.Cause)
	}

	if len(sourceNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}

	// The target acknowledged and holds a reserved UE context; it must be reclaimed
	// with a UE Context Release Command (TS 38.413 §8.4.2).
	if len(targetSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to the target, got %d", len(targetSender.SentUEContextReleaseCommands))
	}
}

// TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SourceUeContextDetached
// verifies that when no PDU sessions are admitted and the source UE's AMF UE
// context has been detached (e.g. due to a concurrent deregistration), the
// handler does not panic.
func TestHandoverRequestAcknowledge_NoPDUSessionsAdmitted_SourceUeContextDetached(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	targetUe := amfInstance.FindUEByAmfUeNgapID(targetRan, 1)
	if targetUe == nil {
		t.Fatal("target UE not found on target radio")
	}

	sourceUeContext := targetUe.UeContext()
	if sourceUeContext == nil {
		t.Fatal("source AMF UE not found")
	}

	sourceUeContext.Conn().AMFForTest().ReleaseNasConnection(sourceUeContext, nil)

	amfID := ngap.AMFUENGAPID(1)
	ranID := ngap.RANUENGAPID(2)
	msg := ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        &amfID,
		RANUENGAPID:                        &ranID,
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0x01, 0x02, 0x03},
	}

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, &msg)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure on source radio, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	if len(sourceNGAPSender.SentHandoverCommands) != 0 {
		t.Fatalf("expected no HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}
}

func TestHandoverRequestAcknowledge_HappyPath(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	transferBytes, err := (&ngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: ngap.UPTransportLayerInformation{
			GTPTunnel: ngap.GTPTunnel{
				TransportLayerAddress: ngap.TransportLayerAddress{10, 0, 0, 2},
				GTPTEID:               0x000004D2,
			},
		},
		QosFlowSetupResponse: ngap.QosFlowListWithDataForwarding{
			{QosFlowIdentifier: 1},
		},
	}).Marshal()
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequestAcknowledgeTransfer: %v", err)
	}

	containerData := []byte{0xAA, 0xBB, 0xCC}

	amfID := ngap.AMFUENGAPID(1)
	ranID := ngap.RANUENGAPID(2)
	msg := ngap.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		PDUSessionResourceAdmittedList: ngap.PDUSessionResourceAdmittedList{
			{PDUSessionID: 1, Transfer: transferBytes},
		},
		TargetToSourceTransparentContainer: containerData,
	}

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, &msg)

	if len(sourceNGAPSender.SentHandoverCommands) != 1 {
		t.Fatalf("expected 1 HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}

	cmd := sourceNGAPSender.SentHandoverCommands[0]
	if cmd.AMFUENGAPID != 100 {
		t.Errorf("expected AmfUeNgapID=100, got %d", cmd.AMFUENGAPID)
	}

	if cmd.RANUENGAPID != 10 {
		t.Errorf("expected RanUeNgapID=10 (source), got %d", cmd.RANUENGAPID)
	}

	if len(cmd.TargetToSourceTransparentContainer) != len(containerData) {
		t.Errorf("expected container length %d, got %d", len(containerData), len(cmd.TargetToSourceTransparentContainer))
	}

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}
}

func admittedAckMsg(t *testing.T) *ngap.HandoverRequestAcknowledge {
	t.Helper()

	transferBytes, err := (&ngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: ngap.UPTransportLayerInformation{GTPTunnel: ngap.GTPTunnel{
			TransportLayerAddress: ngap.TransportLayerAddress{10, 0, 0, 2},
			GTPTEID:               1234,
		}},
		QosFlowSetupResponse: ngap.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequestAcknowledgeTransfer: %v", err)
	}

	amfID := ngap.AMFUENGAPID(1)
	ranID := ngap.RANUENGAPID(2)

	return &ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        &amfID,
		RANUENGAPID:                        &ranID,
		PDUSessionResourceAdmittedList:     ngap.PDUSessionResourceAdmittedList{{PDUSessionID: 1, Transfer: transferBytes}},
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0xAA},
	}
}

// TestHandoverRequestAcknowledge_DuplicateWhilePrepared_Dropped verifies that a
// duplicate/out-of-order acknowledge arriving while the handover has advanced past
// preparation (hoPrepared) is dropped without disturbing the in-flight handover —
// local error handling, not a release (TS 38.413 §10.4).
func TestHandoverRequestAcknowledge_DuplicateWhilePrepared_Dropped(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)
	targetSender := targetRan.Conn.(*fakeNGAPSender)
	msg := admittedAckMsg(t)

	// First acknowledge advances hoPreparing→hoPrepared and sends the HANDOVER COMMAND.
	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentHandoverCommands) != 1 {
		t.Fatalf("expected 1 HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}

	// Second (duplicate) acknowledge lands while the handover is legitimately in
	// progress at hoPrepared; it must be dropped, NOT release the in-flight target.
	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, msg)

	if len(targetSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("a duplicate acknowledge during an in-flight handover must not release the target, got %d releases", len(targetSender.SentUEContextReleaseCommands))
	}
}

// TestHandoverRequestAcknowledge_PartialAdmission verifies that when the target
// admits some PDU sessions and fails others, the Handover Command confirms the
// admitted ones in the Handover List and lists the failed ones in the PDU
// Session Resource To Release List (TS 38.413).
func TestHandoverRequestAcknowledge_PartialAdmission(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	admittedBytes, err := (&ngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: ngap.UPTransportLayerInformation{GTPTunnel: ngap.GTPTunnel{
			TransportLayerAddress: ngap.TransportLayerAddress{10, 0, 0, 2},
			GTPTEID:               1234,
		}},
		QosFlowSetupResponse: ngap.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatalf("failed to marshal admitted transfer: %v", err)
	}

	unsuccessfulBytes, err := (&ngap.HandoverResourceAllocationUnsuccessfulTransfer{
		Cause: ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioResourcesNotAvailable},
	}).Marshal()
	if err != nil {
		t.Fatalf("failed to marshal unsuccessful transfer: %v", err)
	}

	amfID := ngap.AMFUENGAPID(1)
	ranID := ngap.RANUENGAPID(2)
	msg := ngap.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		PDUSessionResourceAdmittedList: ngap.PDUSessionResourceAdmittedList{
			{PDUSessionID: 1, Transfer: admittedBytes},
		},
		PDUSessionResourceFailedToSetup: ngap.PDUSessionResourceFailedToSetupListHOAck{
			{PDUSessionID: 2, Transfer: unsuccessfulBytes},
		},
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0xAA, 0xBB, 0xCC},
	}

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, &msg)

	if len(sourceNGAPSender.SentHandoverCommands) != 1 {
		t.Fatalf("expected 1 HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}

	cmd := sourceNGAPSender.SentHandoverCommands[0]

	if len(cmd.PDUSessionResourceHandoverList) != 1 || cmd.PDUSessionResourceHandoverList[0].PDUSessionID != 1 {
		t.Errorf("expected handover list to confirm session 1, got %+v", cmd.PDUSessionResourceHandoverList)
	}

	if len(cmd.PDUSessionResourceToReleaseList) != 1 || cmd.PDUSessionResourceToReleaseList[0].PDUSessionID != 2 {
		t.Fatalf("expected to-release list to contain session 2 (TS 38.413), got %+v", cmd.PDUSessionResourceToReleaseList)
	}

	// The to-release item must carry a decodable HandoverPreparationUnsuccessfulTransfer.
	relayed, err := ngap.ParseHandoverPreparationUnsuccessfulTransfer(cmd.PDUSessionResourceToReleaseList[0].Transfer)
	if err != nil {
		t.Fatalf("to-release transfer does not decode: %v", err)
	}

	wantRadioNetworkCause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioResourcesNotAvailable}
	if relayed.Cause != wantRadioNetworkCause {
		t.Errorf("cause = %v, want radio-resources-not-available", relayed.Cause)
	}
}
