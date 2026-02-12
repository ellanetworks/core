// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

type HandoverRequiredOpts struct {
	AMFUENGAPID ngapType.AMFUENGAPID
	RANUENGAPID ngapType.RANUENGAPID

	TargetID                           *ngapType.TargetID
	PDUSessionResourceListHORqd        *ngapType.PDUSessionResourceListHORqd
	SourceToTargetTransparentContainer *ngapType.SourceToTargetTransparentContainer
}

func buildHandoverRequired(opts *HandoverRequiredOpts) (ngapType.NGAPPDU, error) {
	if opts == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("HandoverRequiredOpts is nil")
	}

	pdu := ngapType.NGAPPDU{}
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeHandoverPreparation
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentHandoverRequired
	initiatingMessage.Value.HandoverRequired = new(ngapType.HandoverRequired)

	handoverRequired := initiatingMessage.Value.HandoverRequired
	handoverRequiredIEs := &handoverRequired.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = opts.AMFUENGAPID.Value
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = opts.RANUENGAPID.Value
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// HandoverType
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)
	ie.Value.HandoverType.Value = ngapType.HandoverTypePresentIntra5gs
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// Cause
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentCause
	ie.Value.Cause = new(ngapType.Cause)
	ie.Value.Cause.Present = ngapType.CausePresentRadioNetwork
	ie.Value.Cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
	ie.Value.Cause.RadioNetwork.Value = ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// TargetID
	if opts.TargetID != nil {
		ie = ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDTargetID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentTargetID
		ie.Value.TargetID = opts.TargetID
		handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)
	}

	// PDUSessionResourceListHORqd
	if opts.PDUSessionResourceListHORqd != nil {
		ie = ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceListHORqd
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentPDUSessionResourceListHORqd
		ie.Value.PDUSessionResourceListHORqd = opts.PDUSessionResourceListHORqd
		handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)
	}

	// SourceToTargetTransparentContainer
	if opts.SourceToTargetTransparentContainer != nil {
		ie = ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSourceToTargetTransparentContainer
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentSourceToTargetTransparentContainer
		ie.Value.SourceToTargetTransparentContainer = opts.SourceToTargetTransparentContainer
		handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)
	}

	return pdu, nil
}

func TestHandoverRequired(t *testing.T) {
	const (
		targetGnbID  = "000102"
		pduSessionID = uint8(1)
		supi         = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	// Encode a minimal HandoverRequiredTransfer (all optional fields)
	hoRequiredTransfer := ngapType.HandoverRequiredTransfer{}

	hoRequiredTransferBytes, err := aper.MarshalWithParams(hoRequiredTransfer, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequiredTransfer: %v", err)
	}

	// Build TargetID pointing to target gNB
	plmnID, err := getMccAndMncInOctets("001", "01")
	if err != nil {
		t.Fatalf("failed to get PLMN ID octets: %v", err)
	}

	targetGnbBitString := ngapConvert.HexToBitString(targetGnbID, 24)
	targetID := &ngapType.TargetID{
		Present: ngapType.TargetIDPresentTargetRANNodeID,
		TargetRANNodeID: &ngapType.TargetRANNodeID{
			GlobalRANNodeID: ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
				GlobalGNBID: &ngapType.GlobalGNBID{
					PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
					GNBID: ngapType.GNBID{
						Present: ngapType.GNBIDPresentGNBID,
						GNBID:   &targetGnbBitString,
					},
				},
			},
			SelectedTAI: ngapType.TAI{
				PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
				TAC:          ngapType.TAC{Value: aper.OctetString{0x00, 0x00, 0x01}},
			},
		},
	}

	// Build PDUSessionResourceListHORqd with one session
	pduSessionList := &ngapType.PDUSessionResourceListHORqd{
		List: []ngapType.PDUSessionResourceItemHORqd{
			{
				PDUSessionID:             ngapType.PDUSessionID{Value: int64(pduSessionID)},
				HandoverRequiredTransfer: hoRequiredTransferBytes,
			},
		},
	}

	// Build SourceToTargetTransparentContainer (opaque, passed through)
	sourceToTargetContainer := &ngapType.SourceToTargetTransparentContainer{
		Value: []byte{0x01, 0x02, 0x03},
	}

	// Build the HandoverRequired NGAP message
	msg, err := buildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID:                        ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID:                        ngapType.RANUENGAPID{Value: 1},
		TargetID:                           targetID,
		PDUSessionResourceListHORqd:        pduSessionList,
		SourceToTargetTransparentContainer: sourceToTargetContainer,
	})
	if err != nil {
		t.Fatalf("failed to build HandoverRequired: %v", err)
	}

	// Initialize SMF context with a matching SM context
	smfContext.InitializeSMF(nil)

	smf := smfContext.SMFSelf()
	smCtx := smf.NewSMContext(supi, pduSessionID, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &models.SmPolicyData{
		Ambr: &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		QosData: &models.QosData{
			QFI:    1,
			Var5qi: 9, Arp: &models.Arp{
				PriorityLevel: 8,
			},
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

	// Set up the AmfUe with valid security context
	amfUe := amfContext.NewAmfUe()
	amfUe.Supi = supi
	amfUe.SecurityContextAvailable = true
	amfUe.NgKsi.Ksi = 1
	amfUe.MacFailed = false
	amfUe.Kamf = kamfHex
	amfUe.NH = make([]byte, 32)
	amfUe.Ambr = &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[pduSessionID] = &amfContext.SmContext{
		Ref:    smfContext.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	// Set up source RAN and source UE
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	sourceUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = sourceUe
	sourceRan.RanUEs[1] = sourceUe

	// Set up target RAN with matching GNB ID
	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: targetNGAPSender,
		RanPresent: amfContext.RanPresentGNbID,
		RanID: &models.GlobalRanNodeID{
			GNbID: &models.GNbID{
				GNBValue:  targetGnbID,
				BitLength: 24,
			},
		},
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	// Set up AMF with target RAN in Radios map
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc: "001",
				Mnc: "01",
				Sst: 1,
			},
		},
		Radios: map[*sctp.SCTPConn]*amfContext.Radio{
			new(sctp.SCTPConn): targetRan,
		},
	}

	ngap.HandleHandoverRequired(context.Background(), amf, sourceRan, msg.InitiatingMessage.Value.HandoverRequired)

	// Verify a HandoverRequest was sent to the target gNB
	if len(targetNGAPSender.SentHandoverRequests) != 1 {
		t.Fatalf("expected 1 HandoverRequest to target gNB, got %d", len(targetNGAPSender.SentHandoverRequests))
	}
}

func TestHandoverRequired_MissingMandatoryIEs(t *testing.T) {
	// Build a HandoverRequired message without TargetID, PDUSessionResourceListHORqd,
	// or SourceToTargetTransparentContainer. The handler should send an ErrorIndication
	// with criticality diagnostics listing the missing IEs.
	msg, err := buildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID: ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: ngapType.RANUENGAPID{Value: 1},
		// TargetID, PDUSessionResourceListHORqd, SourceToTargetTransparentContainer all nil
	})
	if err != nil {
		t.Fatalf("failed to build HandoverRequired: %v", err)
	}

	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	amf := &amfContext.AMF{}

	ngap.HandleHandoverRequired(context.Background(), amf, ran, msg.InitiatingMessage.Value.HandoverRequired)

	if len(fakeNGAPSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}

	errorIndication := fakeNGAPSender.SentErrorIndications[0]
	if errorIndication.CriticalityDiagnostics == nil {
		t.Fatal("expected CriticalityDiagnostics in ErrorIndication, got nil")
	}

	// Should report 3 missing IEs: TargetID, PDUSessionResourceListHORqd, SourceToTargetTransparentContainer
	ieList := errorIndication.CriticalityDiagnostics.IEsCriticalityDiagnostics
	if ieList == nil || len(ieList.List) != 3 {
		count := 0
		if ieList != nil {
			count = len(ieList.List)
		}

		t.Fatalf("expected 3 missing IE diagnostics, got %d", count)
	}

	if len(fakeNGAPSender.SentHandoverRequests) != 0 {
		t.Fatalf("expected no HandoverRequest to be sent, got %d", len(fakeNGAPSender.SentHandoverRequests))
	}
}

func TestHandoverRequired_UnknownRanUeNgapID(t *testing.T) {
	// Build a valid HandoverRequired message but with a RAN UE NGAP ID
	// that doesn't exist in the RAN's UE map. The handler should send
	// an ErrorIndication with UnknownLocalUENGAPID cause.
	plmnID, err := getMccAndMncInOctets("001", "01")
	if err != nil {
		t.Fatalf("failed to get PLMN ID octets: %v", err)
	}

	targetGnbBitString := ngapConvert.HexToBitString("000102", 24)
	targetID := &ngapType.TargetID{
		Present: ngapType.TargetIDPresentTargetRANNodeID,
		TargetRANNodeID: &ngapType.TargetRANNodeID{
			GlobalRANNodeID: ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
				GlobalGNBID: &ngapType.GlobalGNBID{
					PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
					GNBID: ngapType.GNBID{
						Present: ngapType.GNBIDPresentGNBID,
						GNBID:   &targetGnbBitString,
					},
				},
			},
			SelectedTAI: ngapType.TAI{
				PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
				TAC:          ngapType.TAC{Value: aper.OctetString{0x00, 0x00, 0x01}},
			},
		},
	}

	msg, err := buildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID: ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: ngapType.RANUENGAPID{Value: 99}, // No UE with this ID
		TargetID:    targetID,
		PDUSessionResourceListHORqd: &ngapType.PDUSessionResourceListHORqd{
			List: []ngapType.PDUSessionResourceItemHORqd{
				{PDUSessionID: ngapType.PDUSessionID{Value: 1}, HandoverRequiredTransfer: []byte{0x00}},
			},
		},
		SourceToTargetTransparentContainer: &ngapType.SourceToTargetTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})
	if err != nil {
		t.Fatalf("failed to build HandoverRequired: %v", err)
	}

	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe), // Empty — no UE with ID 99
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	amf := &amfContext.AMF{}

	ngap.HandleHandoverRequired(context.Background(), amf, ran, msg.InitiatingMessage.Value.HandoverRequired)

	if len(fakeNGAPSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}

	errorIndication := fakeNGAPSender.SentErrorIndications[0]
	if errorIndication.Cause == nil {
		t.Fatal("expected Cause in ErrorIndication, got nil")
	}

	if errorIndication.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got present=%d", errorIndication.Cause.Present)
	}

	if errorIndication.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID cause, got %d", errorIndication.Cause.RadioNetwork.Value)
	}
}

func TestHandoverRequired_InvalidSecurityContext(t *testing.T) {
	// Build a valid HandoverRequired but the UE has no valid security context.
	// The handler should send a HandoverPreparationFailure with AuthenticationFailure cause.
	const (
		targetGnbID  = "000102"
		pduSessionID = uint8(1)
	)

	plmnID, err := getMccAndMncInOctets("001", "01")
	if err != nil {
		t.Fatalf("failed to get PLMN ID octets: %v", err)
	}

	targetGnbBitString := ngapConvert.HexToBitString(targetGnbID, 24)
	targetID := &ngapType.TargetID{
		Present: ngapType.TargetIDPresentTargetRANNodeID,
		TargetRANNodeID: &ngapType.TargetRANNodeID{
			GlobalRANNodeID: ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
				GlobalGNBID: &ngapType.GlobalGNBID{
					PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
					GNBID: ngapType.GNBID{
						Present: ngapType.GNBIDPresentGNBID,
						GNBID:   &targetGnbBitString,
					},
				},
			},
			SelectedTAI: ngapType.TAI{
				PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
				TAC:          ngapType.TAC{Value: aper.OctetString{0x00, 0x00, 0x01}},
			},
		},
	}

	msg, err := buildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID: ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID: ngapType.RANUENGAPID{Value: 1},
		TargetID:    targetID,
		PDUSessionResourceListHORqd: &ngapType.PDUSessionResourceListHORqd{
			List: []ngapType.PDUSessionResourceItemHORqd{
				{PDUSessionID: ngapType.PDUSessionID{Value: int64(pduSessionID)}, HandoverRequiredTransfer: []byte{0x00}},
			},
		},
		SourceToTargetTransparentContainer: &ngapType.SourceToTargetTransparentContainer{
			Value: []byte{0x01, 0x02, 0x03},
		},
	})
	if err != nil {
		t.Fatalf("failed to build HandoverRequired: %v", err)
	}

	// Create AmfUe with invalid security context
	amfUe := amfContext.NewAmfUe()
	amfUe.SecurityContextAvailable = false
	amfUe.Log = logger.AmfLog

	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	sourceUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = sourceUe
	sourceRan.RanUEs[1] = sourceUe

	amf := &amfContext.AMF{}

	ngap.HandleHandoverRequired(context.Background(), amf, sourceRan, msg.InitiatingMessage.Value.HandoverRequired)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	failure := sourceNGAPSender.SentHandoverPreparationFailures[0]
	if failure.Cause.Present != ngapType.CausePresentNas {
		t.Fatalf("expected NAS cause, got present=%d", failure.Cause.Present)
	}

	if failure.Cause.Nas.Value != ngapType.CauseNasPresentAuthenticationFailure {
		t.Fatalf("expected AuthenticationFailure cause, got %d", failure.Cause.Nas.Value)
	}

	if len(sourceNGAPSender.SentHandoverRequests) != 0 {
		t.Fatalf("expected no HandoverRequest to be sent, got %d", len(sourceNGAPSender.SentHandoverRequests))
	}
}
