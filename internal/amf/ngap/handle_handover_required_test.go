// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

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

const handoverTargetGnbID = "000102"

func handoverRequired(t *testing.T, ranID ngap.RANUENGAPID, sessions ...uint8) *ngap.HandoverRequired {
	t.Helper()

	gnb, err := hex.DecodeString(handoverTargetGnbID)
	if err != nil || len(gnb) != 3 {
		t.Fatalf("target gNB ID %q is not three octets: %v", handoverTargetGnbID, err)
	}

	transfer, err := (&ngap.HandoverRequiredTransfer{}).Marshal()
	if err != nil {
		t.Fatalf("failed to marshal HandoverRequiredTransfer: %v", err)
	}

	list := make(ngap.PDUSessionResourceListHORqd, 0, len(sessions))
	for _, id := range sessions {
		list = append(list, ngap.PDUSessionResourceItemHORqd{
			PDUSessionID: ngap.PDUSessionID(id),
			Transfer:     transfer,
		})
	}

	cause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHandoverDesirableForRadio}

	return &ngap.HandoverRequired{
		AMFUENGAPID:  1,
		RANUENGAPID:  ranID,
		HandoverType: ngap.HandoverTypeIntra5GS,
		Cause:        &cause,
		TargetID: ngap.TargetID{TargetRANNodeID: &ngap.TargetRANNodeID{
			GlobalRANNodeID: ngap.GlobalRANNodeID{
				Kind:         ngap.RANNodeIDGNB,
				PLMNIdentity: operatorPLMN,
				Value:        uint32(gnb[0])<<16 | uint32(gnb[1])<<8 | uint32(gnb[2]),
				Bits:         24,
			},
			SelectedTAI: ngap.TAI{PLMNIdentity: operatorPLMN, TAC: 1},
		}},
		PDUSessionResourceListHORqd:        list,
		SourceToTargetTransparentContainer: ngap.SourceToTargetTransparentContainer{0x01, 0x02, 0x03},
	}
}

// TS 38.413 §9.2.3.1
func TestHandoverRequired(t *testing.T) {
	for _, tt := range []struct {
		name      string
		withCause bool
	}{
		{"with Cause", true},
		{"without Cause", false},
	} {
		t.Run(tt.name, func(t *testing.T) { testHandoverRequired(t, tt.withCause) })
	}
}

func testHandoverRequired(t *testing.T, withCause bool) {
	t.Helper()

	const (
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	msg := handoverRequired(t, 1, pduSessionID)
	if !withCause {
		msg.Cause = nil
	}

	smfInstance := smf.New(nil, nil, nil, nil)

	smCtx, _ := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr: models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")},
		QosData: models.QosData{
			QFI:    1,
			Var5qi: 9, Arp: &models.Arp{
				PriorityLevel: 8,
			},
		},
	}
	smCtx.Tunnel = &smf.UPTunnel{N3TEID: 1234, N3IPv4: netip.MustParseAddr("10.0.0.1")}

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
	amfUe.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smCtx.Ref,
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

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       targetNGAPSender,
		RanPresent: amf.RanPresentGNbID,
		RanID: &models.GlobalRanNodeID{
			GNbID: &models.GNbID{
				GNBValue:  handoverTargetGnbID,
				BitLength: 24,
			},
		},
	}

	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

	if len(targetNGAPSender.SentHandoverRequests) != 1 {
		t.Fatalf("expected 1 HandoverRequest to target gNB, got %d", len(targetNGAPSender.SentHandoverRequests))
	}

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}
}

func TestHandoverRequired_UnknownRanUeNgapID(t *testing.T) {
	msg := handoverRequired(t, 99, 1)

	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	amfInstance := amf.New(nil, nil, nil)

	HandleHandoverRequired(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errorIndication := sender.SentErrorIndications[0]
	if errorIndication.Cause == nil {
		t.Fatal("expected Cause in ErrorIndication, got nil")
	}

	wantRadioNetworkCause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID}
	if errorIndication.Cause == nil || *errorIndication.Cause != wantRadioNetworkCause {
		t.Errorf("cause = %v, want unknown-local-UE-NGAP-ID", errorIndication.Cause)
	}
}

func TestHandoverRequired_InvalidSecurityContext(t *testing.T) {
	const (
		pduSessionID = uint8(1)
	)

	msg := handoverRequired(t, 1, pduSessionID)

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

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	failure := sourceNGAPSender.SentHandoverPreparationFailures[0]

	wantNasCause := ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASAuthenticationFailure}
	if failure.Cause == nil || *failure.Cause != wantNasCause {
		t.Errorf("cause = %v, want authentication-failure", failure.Cause)
	}

	if len(sourceNGAPSender.SentHandoverRequests) != 0 {
		t.Fatalf("expected no HandoverRequest to be sent, got %d", len(sourceNGAPSender.SentHandoverRequests))
	}
}

func TestHandoverRequired_UnknownTarget(t *testing.T) {
	const (
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	msg := handoverRequired(t, 1, pduSessionID)

	smfInstance := smf.New(nil, nil, nil, nil)
	smCtx, _ := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, &models.Snssai{Sst: 1})

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smCtx.Ref,
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

	amfInstance.ClearRadiosForTest()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

	if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
	}

	failure := sourceNGAPSender.SentHandoverPreparationFailures[0]

	wantRadioNetworkCause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownTargetID}
	if failure.Cause == nil || *failure.Cause != wantRadioNetworkCause {
		t.Errorf("cause = %v, want unknown-targetID", failure.Cause)
	}
}

func TestHandoverRequired_GuardExpiryReleasesTarget(t *testing.T) {
	const (
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	msg := handoverRequired(t, 1, pduSessionID)

	smfInstance := smf.New(nil, nil, nil, nil)

	smCtx, _ := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")},
		QosData: models.QosData{QFI: 1, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 8}},
	}
	smCtx.Tunnel = &smf.UPTunnel{N3TEID: 1234, N3IPv4: netip.MustParseAddr("10.0.0.1")}

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
	amfUe.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{
		Ref:    smCtx.Ref,
		Snssai: &models.Snssai{Sst: 1},
	}

	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: &fakeNGAPSender{},
	}

	smfSbi := &fakeSmfSbi{SMF: smfInstance}
	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, smfSbi)
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetSender := &releaseSignalSender{fakeNGAPSender: targetNGAPSender, released: make(chan struct{})}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       targetSender,
		RanPresent: amf.RanPresentGNbID,
		RanID:      &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: handoverTargetGnbID, BitLength: 24}},
	}

	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	amfInstance.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

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

	procDeadline := time.Now().Add(2 * time.Second)
	for amfUe.Procedures().Active(procedure.N2Handover) {
		if time.Now().After(procDeadline) {
			t.Fatal("N2Handover procedure still active after guard expiry")
		}

		time.Sleep(time.Millisecond)
	}

	wantRef := smCtx.Ref
	if got := smfSbi.N2HandoverCanceledCalls; len(got) != 1 || got[0] != wantRef {
		t.Fatalf("source access tunnel not restored on guard expiry: N2HandoverCanceled calls = %v, want [%s]", got, wantRef)
	}
}

func TestHandoverRequired_SourceDropReleasesTarget(t *testing.T) {
	const (
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	msg := handoverRequired(t, 1, pduSessionID)

	smfInstance := smf.New(nil, nil, nil, nil)

	smCtx, _ := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")},
		QosData: models.QosData{QFI: 1, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 8}},
	}
	smCtx.Tunnel = &smf.UPTunnel{N3TEID: 1234, N3IPv4: netip.MustParseAddr("10.0.0.1")}

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))

	amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
	amfUe.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{Ref: smCtx.Ref, Snssai: &models.Snssai{Sst: 1}}

	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}
	smfSbi := &fakeSmfSbi{SMF: smfInstance}
	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, smfSbi)
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       targetNGAPSender,
		RanPresent: amf.RanPresentGNbID,
		RanID:      &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: handoverTargetGnbID, BitLength: 24}},
	}
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

	if len(targetNGAPSender.SentHandoverRequests) != 1 {
		t.Fatalf("expected 1 HandoverRequest to target gNB, got %d", len(targetNGAPSender.SentHandoverRequests))
	}

	if !amfUe.Procedures().Active(procedure.N2Handover) {
		t.Fatal("N2Handover procedure not active after preparation")
	}

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

	wantRef := smCtx.Ref
	if got := smfSbi.N2HandoverCanceledCalls; len(got) != 1 || got[0] != wantRef {
		t.Fatalf("source access tunnel not restored on source association removal: N2HandoverCanceled calls = %v, want [%s]", got, wantRef)
	}
}

// TS 38.413 §9.2.3.2
func TestHandoverRequired_UnsupportedHandoverType(t *testing.T) {
	const (
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	for _, ht := range []ngap.HandoverType{ngap.HandoverTypeEPSToFiveGS} {
		t.Run(fmt.Sprintf("handoverType %d", ht), func(t *testing.T) {
			supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

			msg := handoverRequired(t, 1, pduSessionID)
			msg.HandoverType = ht

			smfInstance := smf.New(nil, nil, nil, nil)
			smCtx, _ := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, &models.Snssai{Sst: 1})

			amfUe := amf.NewUeContext()
			amfUe.SetSupiForTest(supi)
			amfUe.SetSecuredForTest(true)
			amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
			amfUe.SetKamfForTest(kamfHex)
			amfUe.SetNHForTest(make([]byte, 32))
			amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
			amfUe.SmContextList[pduSessionID] = &amf.SmContext{
				Ref:    smCtx.Ref,
				Snssai: &models.Snssai{Sst: 1},
			}

			sourceNGAPSender := &fakeNGAPSender{}
			sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: sourceNGAPSender}
			amfInstance := amf.New(&fakeDBInstance{
				Operator: &db.Operator{Mcc: "001", Mnc: "01"},
			}, nil, &fakeSmfSbi{SMF: smfInstance})
			sourceRan.BindAMFForTest(amfInstance)

			sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
			sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

			HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

			if len(sourceNGAPSender.SentHandoverPreparationFailures) != 1 {
				t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceNGAPSender.SentHandoverPreparationFailures))
			}

			want := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHoTargetNotAllowed}
			if failure := sourceNGAPSender.SentHandoverPreparationFailures[0]; failure.Cause == nil || *failure.Cause != want {
				t.Errorf("cause = %v, want ho-target-not-allowed", failure.Cause)
			}

			if len(sourceNGAPSender.SentHandoverCommands) != 0 {
				t.Errorf("sent %d HandoverCommands, want 0", len(sourceNGAPSender.SentHandoverCommands))
			}
		})
	}
}

func TestHandoverRequired_AbandonedTargetReleaseKeepsSessionsActive(t *testing.T) {
	const (
		pduSessionID = uint8(1)
		supiStr      = "imsi-001010000000001"
		dnn          = "internet"
		kamfHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	)

	supi, _ := etsi.NewSUPIFromPrefixed(supiStr)

	msg := handoverRequired(t, 1, pduSessionID)

	smfInstance := smf.New(nil, nil, nil, nil)

	smCtx, _ := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, dnn, &models.Snssai{Sst: 1})
	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")},
		QosData: models.QosData{QFI: 1, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 8}},
	}
	smCtx.Tunnel = &smf.UPTunnel{N3TEID: 1234, N3IPv4: netip.MustParseAddr("10.0.0.1")}

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest(kamfHex)
	amfUe.SetNHForTest(make([]byte, 32))
	amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
	amfUe.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{Ref: smCtx.Ref, Snssai: &models.Snssai{Sst: 1}}

	amfUe.TransitionTo(amf.RegistrationInitiated)
	amfUe.TransitionTo(amf.Registered)

	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}
	smfSbi := &fakeSmfSbi{SMF: smfInstance}
	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, smfSbi)
	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       &fakeNGAPSender{},
		RanPresent: amf.RanPresentGNbID,
		RanID:      &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: handoverTargetGnbID, BitLength: 24}},
	}
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

	targetUe := amfInstance.HandoverTarget(amfUe)
	if targetUe == nil {
		t.Fatal("no target UeConn staged by handover preparation")
	}

	if err := amfInstance.RemoveUeConn(context.Background(), sourceUe); err != nil {
		t.Fatalf("RemoveUeConn(source) error: %v", err)
	}

	if got := targetUe.UeContext(); got != nil {
		t.Error("abandoned target still bound to the UE context")
	}

	amfInstance.ReleaseUeConn(context.Background(), targetUe)

	if got := smfSbi.DeactivateSmContextCalls; len(got) != 0 {
		t.Fatalf("releasing an abandoned handover target deactivated the UE's sessions: DeactivateSmContext calls = %v, want none", got)
	}
}
