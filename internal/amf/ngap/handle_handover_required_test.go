// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

// releaseSignalSender wraps a sender and closes released the first time a
// UE Context Release Command is sent, giving a test a happens-before edge to the
// guard's timer goroutine.
type releaseSignalSender struct {
	*fakeNGAPSender
	released chan struct{}
}

func (s *releaseSignalSender) WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error) {
	before := len(s.SentUEContextReleaseCommands)

	n, err := s.fakeNGAPSender.WriteMsg(b, info)

	if before == 0 && len(s.SentUEContextReleaseCommands) > 0 {
		close(s.released)
	}

	return n, err
}

// decodeHandoverRequiredOrFatal decodes msg and fails the test only if
// the decoder reports a fatal error. Non-fatal reports are accepted:
// the dispatcher would invoke the handler in that case anyway.
func decodeHandoverRequiredOrFatal(t *testing.T, msg *ngapType.HandoverRequired) decode.HandoverRequired {
	t.Helper()

	decoded, report := decode.DecodeHandoverRequired(msg)
	if report != nil && report.Fatal() {
		t.Fatalf("decoder produced fatal report: %+v", report)
	}

	return decoded
}

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

	ie := ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = opts.AMFUENGAPID.Value
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = opts.RANUENGAPID.Value
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)
	ie.Value.HandoverType.Value = ngapType.HandoverTypePresentIntra5gs
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentCause
	ie.Value.Cause = new(ngapType.Cause)
	ie.Value.Cause.Present = ngapType.CausePresentRadioNetwork
	ie.Value.Cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
	ie.Value.Cause.RadioNetwork.Value = ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	if opts.TargetID != nil {
		ie = ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDTargetID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentTargetID
		ie.Value.TargetID = opts.TargetID
		handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)
	}

	if opts.PDUSessionResourceListHORqd != nil {
		ie = ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceListHORqd
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentPDUSessionResourceListHORqd
		ie.Value.PDUSessionResourceListHORqd = opts.PDUSessionResourceListHORqd
		handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)
	}

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
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	// HandoverRequiredTransfer fields are all optional, so an empty value is valid.
	hoRequiredTransfer := ngapType.HandoverRequiredTransfer{}

	hoRequiredTransferBytes, err := aper.MarshalWithParams(hoRequiredTransfer, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequiredTransfer: %v", err)
	}

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

	pduSessionList := &ngapType.PDUSessionResourceListHORqd{
		List: []ngapType.PDUSessionResourceItemHORqd{
			{
				PDUSessionID:             ngapType.PDUSessionID{Value: int64(pduSessionID)},
				HandoverRequiredTransfer: hoRequiredTransferBytes,
			},
		},
	}

	// SourceToTargetTransparentContainer is opaque and passed through unchanged.
	sourceToTargetContainer := &ngapType.SourceToTargetTransparentContainer{
		Value: []byte{0x01, 0x02, 0x03},
	}

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

	smfInstance := smf.New(nil, nil, nil, nil)

	smCtx := smfInstance.NewSession(supi, pduSessionID, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr: models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		QosData: models.QosData{
			QFI:    1,
			Var5qi: 9, Arp: &models.Arp{
				PriorityLevel: 8,
			},
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
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	secCap := &nasType.UESecurityCapability{}
	secCap.SetLen(2)
	amfUe.SetUESecurityCapabilityForTest(secCap)
	amfUe.Ambr = &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smf.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, &fakeSmfSbi{SMF: smfInstance})
	sourceRan.BindAMFForTest(amfInstance)
	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	// Target RAN's GNB ID matches the HandoverRequired TargetID, so the AMF routes to it.
	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       targetNGAPSender,
		RanPresent: amf.RanPresentGNbID,
		RanID: &models.GlobalRanNodeID{
			GNbID: &models.GNbID{
				GNBValue:  targetGnbID,
				BitLength: 24,
			},
		},
	}

	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	ngap.HandleHandoverRequired(context.Background(), amfInstance, sourceRan, decodeHandoverRequiredOrFatal(t, msg.InitiatingMessage.Value.HandoverRequired))

	if len(targetNGAPSender.SentHandoverRequests) != 1 {
		t.Fatalf("expected 1 HandoverRequest to target gNB, got %d", len(targetNGAPSender.SentHandoverRequests))
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

	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	amfInstance := amf.New(nil, nil, nil)

	ngap.HandleHandoverRequired(context.Background(), amfInstance, ran, decodeHandoverRequiredOrFatal(t, msg.InitiatingMessage.Value.HandoverRequired))

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errorIndication := sender.SentErrorIndications[0]
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

	amfUe := amf.NewUeContext()
	amfUe.SetSecuredForTest(false)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}
	amfInstance := amf.New(nil, nil, nil)
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	ngap.HandleHandoverRequired(context.Background(), amfInstance, sourceRan, decodeHandoverRequiredOrFatal(t, msg.InitiatingMessage.Value.HandoverRequired))

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

// TestHandoverRequired_UnknownTarget verifies that when the target gNB is not
// served by this AMF, handover preparation fails gracefully: the source UE
// receives a HandoverPreparationFailure with cause UnknownTargetID rather than
// being left without a response (TS 38.413).
func TestHandoverRequired_UnknownTarget(t *testing.T) {
	const (
		targetGnbID  = "000102"
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	hoRequiredTransferBytes, err := aper.MarshalWithParams(ngapType.HandoverRequiredTransfer{}, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequiredTransfer: %v", err)
	}

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
				{PDUSessionID: ngapType.PDUSessionID{Value: int64(pduSessionID)}, HandoverRequiredTransfer: hoRequiredTransferBytes},
			},
		},
		SourceToTargetTransparentContainer: &ngapType.SourceToTargetTransparentContainer{Value: []byte{0x01, 0x02, 0x03}},
	})
	if err != nil {
		t.Fatalf("failed to build HandoverRequired: %v", err)
	}

	smfInstance := smf.New(nil, nil, nil, nil)
	smfInstance.NewSession(supi, pduSessionID, dnn, &models.Snssai{Sst: 1})

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	secCap := &nasType.UESecurityCapability{}
	secCap.SetLen(2)
	amfUe.SetUESecurityCapabilityForTest(secCap)
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smf.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01"},
	}, nil, &fakeSmfSbi{SMF: smfInstance})
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	// No target gNB registered with this AMF.
	amfInstance.ClearRadiosForTest()

	ngap.HandleHandoverRequired(context.Background(), amfInstance, sourceRan, decodeHandoverRequiredOrFatal(t, msg.InitiatingMessage.Value.HandoverRequired))

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	failure := sourceNGAPSender.SentHandoverPreparationFailures[0]
	if failure.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got present=%d", failure.Cause.Present)
	}

	if failure.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownTargetID {
		t.Fatalf("expected UnknownTargetID cause, got %d", failure.Cause.RadioNetwork.Value)
	}
}

// TestHandoverRequired_GuardExpiryReleasesTarget drives a normal handover
// preparation (source → AMF → target), then lets the supervision guard expire
// because the target gNB never answers. The guard must abandon the handover:
// release the target's half-prepared UE context and clear the N2Handover
// procedure so it no longer pins the source UE.
func TestHandoverRequired_GuardExpiryReleasesTarget(t *testing.T) {
	const (
		targetGnbID  = "000102"
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	hoRequiredTransferBytes, err := aper.MarshalWithParams(ngapType.HandoverRequiredTransfer{}, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequiredTransfer: %v", err)
	}

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

	pduSessionList := &ngapType.PDUSessionResourceListHORqd{
		List: []ngapType.PDUSessionResourceItemHORqd{
			{
				PDUSessionID:             ngapType.PDUSessionID{Value: int64(pduSessionID)},
				HandoverRequiredTransfer: hoRequiredTransferBytes,
			},
		},
	}

	msg, err := buildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID:                        ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID:                        ngapType.RANUENGAPID{Value: 1},
		TargetID:                           targetID,
		PDUSessionResourceListHORqd:        pduSessionList,
		SourceToTargetTransparentContainer: &ngapType.SourceToTargetTransparentContainer{Value: []byte{0x01, 0x02, 0x03}},
	})
	if err != nil {
		t.Fatalf("failed to build HandoverRequired: %v", err)
	}

	smfInstance := smf.New(nil, nil, nil, nil)

	smCtx := smfInstance.NewSession(supi, pduSessionID, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		QosData: models.QosData{QFI: 1, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 8}},
	}
	smCtx.Tunnel = &smf.UPTunnel{
		DataPath: &smf.DataPath{
			UpLinkTunnel: &smf.GTPTunnel{TEID: 1234, N3IPv4: netip.MustParseAddr("10.0.0.1")},
		},
	}

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	secCap := &nasType.UESecurityCapability{}
	secCap.SetLen(2)
	amfUe.SetUESecurityCapabilityForTest(secCap)
	amfUe.Ambr = &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smf.CanonicalName(supi, pduSessionID),
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: &fakeNGAPSender{},
	}

	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, &fakeSmfSbi{SMF: smfInstance})
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetSender := &releaseSignalSender{fakeNGAPSender: targetNGAPSender, released: make(chan struct{})}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       targetSender,
		RanPresent: amf.RanPresentGNbID,
		RanID:      &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: targetGnbID, BitLength: 24}},
	}

	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	// Drive the guard quickly; the target gNB never answers the HANDOVER REQUEST.
	amfInstance.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	ngap.HandleHandoverRequired(context.Background(), amfInstance, sourceRan, decodeHandoverRequiredOrFatal(t, msg.InitiatingMessage.Value.HandoverRequired))

	if len(targetNGAPSender.SentHandoverRequests) != 1 {
		t.Fatalf("expected 1 HandoverRequest to target gNB, got %d", len(targetNGAPSender.SentHandoverRequests))
	}

	select {
	case <-targetSender.released:
	case <-time.After(2 * time.Second):
		t.Fatal("guard did not release the target gNB UE context within timeout")
	}

	if got := len(targetNGAPSender.SentUEContextReleaseCommands); got != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to target on guard expiry, got %d", got)
	}

	if amfUe.Procedures().Active(procedure.N2Handover) {
		t.Fatal("N2Handover procedure still active after guard expiry")
	}
}

// TestHandoverRequired_UnsupportedTargetIDFailsToSource checks a HANDOVER REQUIRED with a
// validly-decoded but unservable TargetID (not a target RAN node) is answered with
// HANDOVER PREPARATION FAILURE, so the source gNB is not left waiting on its own timer
// (TS 38.413 §8.4.1.3).
func TestHandoverRequired_UnsupportedTargetIDFailsToSource(t *testing.T) {
	supi, _ := etsi.NewSUPIFromPrefixed("imsi-001010000000001")

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")
	amfUe.SetNHForTest(make([]byte, 32))

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: sourceNGAPSender}
	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, &fakeSmfSbi{SMF: smf.New(nil, nil, nil, nil)})
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	// A TargeteNBID target is a validly-decoded choice the AMF cannot serve (it prepares
	// only toward a target RAN node).
	msg := decode.HandoverRequired{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		TargetID:    &ngapType.TargetID{Present: ngapType.TargetIDPresentTargeteNBID},
	}

	ngap.HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure to source, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	if got := sourceNGAPSender.SentHandoverPreparationFailures[0].Cause; got.Present != ngapType.CausePresentRadioNetwork ||
		got.RadioNetwork == nil || got.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentHoTargetNotAllowed {
		t.Fatalf("expected HoTargetNotAllowed RadioNetwork cause, got %+v", got)
	}

	if len(sourceNGAPSender.SentHandoverRequests) != 0 {
		t.Fatalf("expected no HandoverRequest for an unservable target, got %d", len(sourceNGAPSender.SentHandoverRequests))
	}
}

// TestHandoverRequired_SourceDropReleasesTarget drives a normal handover to the prepared
// state, then removes the source association (as a source-gNB SCTP drop would). The
// prepared target must be released immediately rather than lingering until the
// supervision guard, and the N2Handover procedure cleared.
func TestHandoverRequired_SourceDropReleasesTarget(t *testing.T) {
	const (
		targetGnbID  = "000102"
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	hoRequiredTransferBytes, err := aper.MarshalWithParams(ngapType.HandoverRequiredTransfer{}, "valueExt")
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequiredTransfer: %v", err)
	}

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
		AMFUENGAPID:                        ngapType.AMFUENGAPID{Value: 1},
		RANUENGAPID:                        ngapType.RANUENGAPID{Value: 1},
		TargetID:                           targetID,
		PDUSessionResourceListHORqd:        &ngapType.PDUSessionResourceListHORqd{List: []ngapType.PDUSessionResourceItemHORqd{{PDUSessionID: ngapType.PDUSessionID{Value: int64(pduSessionID)}, HandoverRequiredTransfer: hoRequiredTransferBytes}}},
		SourceToTargetTransparentContainer: &ngapType.SourceToTargetTransparentContainer{Value: []byte{0x01, 0x02, 0x03}},
	})
	if err != nil {
		t.Fatalf("failed to build HandoverRequired: %v", err)
	}

	smfInstance := smf.New(nil, nil, nil, nil)

	smCtx := smfInstance.NewSession(supi, pduSessionID, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		QosData: models.QosData{QFI: 1, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 8}},
	}
	smCtx.Tunnel = &smf.UPTunnel{DataPath: &smf.DataPath{UpLinkTunnel: &smf.GTPTunnel{TEID: 1234, N3IPv4: netip.MustParseAddr("10.0.0.1")}}}

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	secCap := &nasType.UESecurityCapability{}
	secCap.SetLen(2)
	amfUe.SetUESecurityCapabilityForTest(secCap)
	amfUe.Ambr = &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{Ref: smf.CanonicalName(supi, pduSessionID), Snssai: &models.Snssai{Sst: 1}}

	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}
	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, &fakeSmfSbi{SMF: smfInstance})
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       targetNGAPSender,
		RanPresent: amf.RanPresentGNbID,
		RanID:      &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: targetGnbID, BitLength: 24}},
	}
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	ngap.HandleHandoverRequired(context.Background(), amfInstance, sourceRan, decodeHandoverRequiredOrFatal(t, msg.InitiatingMessage.Value.HandoverRequired))

	if len(targetNGAPSender.SentHandoverRequests) != 1 {
		t.Fatalf("expected 1 HandoverRequest to target gNB, got %d", len(targetNGAPSender.SentHandoverRequests))
	}

	if !amfUe.Procedures().Active(procedure.N2Handover) {
		t.Fatal("N2Handover procedure not active after preparation")
	}

	// The source association is lost (gNB SCTP drop). The prepared target must be released
	// at once, not left for the 10 s guard.
	if err := amfInstance.RemoveUeConn(context.Background(), sourceUe); err != nil {
		t.Fatalf("RemoveUeConn(source) error: %v", err)
	}

	if got := len(targetNGAPSender.SentUEContextReleaseCommands); got != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to target on source drop, got %d", got)
	}

	if amfUe.Procedures().Active(procedure.N2Handover) {
		t.Fatal("N2Handover procedure still active after source association removal")
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("handover FSM not cleared after source association removal")
	}
}
