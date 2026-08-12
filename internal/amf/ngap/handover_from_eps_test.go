// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
)

const (
	arrivingGNBID          = 0x0abcde
	arrivingPDUSessionID   = 1
	arrivingEPSBearerID    = 5
	arrivingRANUEID        = 42
	arrivingSelectedTAC    = 1
	arrivingRelocationID   = 77
	arrivingTargetToSource = 0xa5
)

type arrivalSender struct {
	*fakeNGAPSender
	requested chan struct{}
	once      sync.Once
}

func (s *arrivalSender) WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error) {
	n, err := s.fakeNGAPSender.WriteMsg(b, info)

	if len(s.SentHandoverRequests) > 0 {
		s.once.Do(func() { close(s.requested) })
	}

	return n, err
}

func newArrivalSender() *arrivalSender {
	return &arrivalSender{fakeNGAPSender: &fakeNGAPSender{}, requested: make(chan struct{})}
}

func (s *arrivalSender) awaitHandoverRequest(t *testing.T) *ngap.HandoverRequest {
	t.Helper()

	select {
	case <-s.requested:
	case <-time.After(2 * time.Second):
		t.Fatal("the target NG-RAN node never got a Handover Request")
	}

	return s.SentHandoverRequests[0]
}

func arrivingOperator() *db.Operator {
	return &db.Operator{
		Mcc:           "001",
		Mnc:           "01",
		SupportedTACs: `["000001"]`,
		Ciphering:     `["AES","SNOW3G","NULL"]`,
		Integrity:     `["AES","SNOW3G"]`,
	}
}

func arrivingTarget() interworking.NGRANIdentity {
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}

	return interworking.NGRANIdentity{
		Kind:        interworking.NGRANNodeGNB,
		PlmnID:      plmn,
		ID:          arrivingGNBID,
		Bits:        24,
		SelectedTAI: interworking.FiveGSTAI{PlmnID: plmn, TAC: arrivingSelectedTAC},
	}
}

func arrivingRequest() interworking.FiveGSRelocationRequest {
	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		panic(err)
	}

	return interworking.FiveGSRelocationRequest{
		ID:   arrivingRelocationID,
		SUPI: supi,
		SecurityContext: interworking.EPSSecurityContext{
			KASME:                [32]byte{0x01, 0x02, 0x03, 0x04},
			EKSI:                 nas.KeySetIdentifier{Value: 3},
			ULNASCount:           nas.Count(4),
			DLNASCount:           nas.Count(6),
			Algorithms:           interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
			UESecurityCapability: eps.UESecurityCapability{EEA: 0xe0, EIA: 0xe0},
			NH:                   [32]byte{0x0a},
			NCC:                  2,
		},
		PDNConnections: []interworking.PDNConnection{{
			PDUSessionID:      arrivingPDUSessionID,
			EPSBearerIdentity: arrivingEPSBearerID,
			APN:               "internet",
			Snssai:            models.Snssai{Sst: 1},
		}},
		Target:         arrivingTarget(),
		SourceToTarget: []byte{0x01, 0x02},
		Cause:          s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkTimeCriticalHandover},
		UEAMBRUplink:   models.MustParseBitRate("1 Gbps"),
		UEAMBRDownlink: models.MustParseBitRate("1 Gbps"),
	}
}

func arrivingAMF(t *testing.T, peer *epsPeerStub) (*amf.AMF, *amf.Radio, *arrivalSender, *fakeSmfSbi) {
	t.Helper()

	sender := newArrivalSender()
	smfSbi := &fakeSmfSbi{PrepareFromEPSResponse: []byte{0x09, 0x08}}

	amfInstance := amf.New(&fakeDBInstance{Operator: arrivingOperator()}, nil, smfSbi)
	amfInstance.EPS = peer

	radio := &amf.Radio{Log: logger.AmfLog, Conn: sender}
	radio.BindAMFForTest(amfInstance)

	target, err := amf.NGRANIdentityToNGAP(arrivingTarget())
	if err != nil {
		t.Fatalf("NGRANIdentityToNGAP: %v", err)
	}

	amfInstance.ClaimRanID(radio, target)

	return amfInstance, radio, sender, smfSbi
}

func arrivingAMFOnly(t *testing.T) *amf.AMF {
	t.Helper()

	amfInstance, _, _, _ := arrivingAMF(t, &epsPeerStub{}) //nolint:dogsled // only the AMF is under test here

	return amfInstance
}

func admitArrival(t *testing.T, amfInstance *amf.AMF, radio *amf.Radio, req *ngap.HandoverRequest, sessions ...ngap.PDUSessionID) {
	t.Helper()

	admitted := make(ngap.PDUSessionResourceAdmittedList, 0, len(sessions))
	for _, pduSessionID := range sessions {
		admitted = append(admitted, ngap.PDUSessionResourceAdmittedItem{
			PDUSessionID: pduSessionID,
			Transfer:     ngap.TransferContainer{0x00},
		})
	}

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, radio, &ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        ngap.Ptr(req.AMFUENGAPID),
		RANUENGAPID:                        ngap.Ptr(ngap.RANUENGAPID(arrivingRANUEID)),
		PDUSessionResourceAdmittedList:     admitted,
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{arrivingTargetToSource},
	})
}

type arrivalOutcome struct {
	resp interworking.FiveGSRelocationResponse
	err  error
}

func forwardArrival(t *testing.T, amfInstance *amf.AMF, req interworking.FiveGSRelocationRequest) <-chan arrivalOutcome {
	t.Helper()

	done := make(chan arrivalOutcome, 1)

	go func() {
		resp, err := amfInstance.ForwardRelocation(context.Background(), req)
		done <- arrivalOutcome{resp: resp, err: err}
	}()

	return done
}

func awaitArrival(t *testing.T, done <-chan arrivalOutcome) (interworking.FiveGSRelocationResponse, error) {
	t.Helper()

	select {
	case out := <-done:
		return out.resp, out.err
	case <-time.After(2 * time.Second):
		t.Fatal("ForwardRelocation never answered the source MME")

		return interworking.FiveGSRelocationResponse{}, nil
	}
}

func TestForwardRelocationSendsAnInterSystemHandoverRequest(t *testing.T) {
	peer := &epsPeerStub{}
	amfInstance, radio, sender, smfSbi := arrivingAMF(t, peer)

	req := arrivingRequest()
	done := forwardArrival(t, amfInstance, req)
	hoReq := sender.awaitHandoverRequest(t)

	if hoReq.HandoverType != ngap.HandoverTypeEPSToFiveGS {
		t.Errorf("handover type = %d, want eps-to-5gs", hoReq.HandoverType)
	}

	if hoReq.NewSecurityContextInd == nil {
		t.Error("the New Security Context Indicator is absent, so the target will not take the mapped context into use")
	}

	if len(hoReq.NASC) == 0 {
		t.Fatal("no S1 mode to N1 mode NAS transparent container for the UE")
	}

	container, err := fgs.ParseS1ModeToN1ModeNASTransparentContainer(hoReq.NASC)
	if err != nil {
		t.Fatalf("parse the NAS container: %v", err)
	}

	if container.NgKSI.Value != req.SecurityContext.EKSI.Value || !container.NgKSI.Mapped {
		t.Errorf("container ngKSI = %+v, want the eKSI %d marked as mapped", container.NgKSI, req.SecurityContext.EKSI.Value)
	}

	if container.NCC != req.SecurityContext.NCC {
		t.Errorf("container NCC = %d, want the %d that keyed K'AMF", container.NCC, req.SecurityContext.NCC)
	}

	if hoReq.SecurityContext.NextHopChainingCount != 0 {
		t.Errorf("NCC to the target = %d, want 0: the pair carries the temporary K_gNB", hoReq.SecurityContext.NextHopChainingCount)
	}

	if hoReq.SecurityContext.NextHopNH == (ngap.SecurityKey{}) {
		t.Error("no next-hop key for the target NG-RAN node")
	}

	if hoReq.Cause == nil || *hoReq.Cause != (ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkTimeCriticalHandover}) {
		t.Errorf("cause = %+v, want the source eNB's time-critical handover mapped through", hoReq.Cause)
	}

	if hoReq.MobilityRestrictionList == nil {
		t.Error("no Mobility Restriction List, so the target cannot determine the serving PLMN")
	}

	if len(hoReq.PDUSessionResourceSetupListHOReq) != 1 {
		t.Fatalf("PDU sessions to set up = %d, want 1", len(hoReq.PDUSessionResourceSetupListHOReq))
	}

	item := hoReq.PDUSessionResourceSetupListHOReq[0]
	if item.PDUSessionID != arrivingPDUSessionID {
		t.Errorf("PDU session id = %d, want %d", item.PDUSessionID, arrivingPDUSessionID)
	}

	if string(item.Transfer) != string(smfSbi.PrepareFromEPSResponse) {
		t.Errorf("the target got transfer % x, want the SMF's % x", item.Transfer, smfSbi.PrepareFromEPSResponse)
	}

	if string(hoReq.SourceToTargetTransparentContainer) != string(req.SourceToTarget) {
		t.Errorf("source-to-target container = % x, want % x", hoReq.SourceToTargetTransparentContainer, req.SourceToTarget)
	}

	if len(smfSbi.PrepareFromEPSCalls) != 1 {
		t.Fatalf("SMF preparations = %d, want 1", len(smfSbi.PrepareFromEPSCalls))
	}

	if got := smfSbi.PrepareFromEPSCalls[0]; got.PDUSessionID != arrivingPDUSessionID || got.EPSBearerIdentity != arrivingEPSBearerID || got.Dnn != "internet" {
		t.Errorf("the SMF was asked for %+v", got)
	}

	admitArrival(t, amfInstance, radio, hoReq, arrivingPDUSessionID)

	resp, err := awaitArrival(t, done)
	if err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	if len(resp.TargetToSource) != 1 || resp.TargetToSource[0] != arrivingTargetToSource {
		t.Errorf("target-to-source container = % x", resp.TargetToSource)
	}

	if len(resp.AcceptedEPSBearers) != 1 || resp.AcceptedEPSBearers[0] != arrivingEPSBearerID {
		t.Errorf("accepted EPS bearers = %v, want [%d]", resp.AcceptedEPSBearers, arrivingEPSBearerID)
	}
}

func TestHandoverNotifyFromEPSPublishesTheUEAndTellsTheMME(t *testing.T) {
	peer := &epsPeerStub{}
	amfInstance, radio, sender, smfSbi := arrivingAMF(t, peer)

	req := arrivingRequest()
	done := forwardArrival(t, amfInstance, req)
	hoReq := sender.awaitHandoverRequest(t)
	admitArrival(t, amfInstance, radio, hoReq, arrivingPDUSessionID)

	if _, err := awaitArrival(t, done); err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	HandleHandoverNotify(context.Background(), amfInstance, radio, &ngap.HandoverNotify{
		AMFUENGAPID: hoReq.AMFUENGAPID,
		RANUENGAPID: arrivingRANUEID,
	})

	ue, ok := amfInstance.LookupUeBySupi(req.SUPI)
	if !ok {
		t.Fatal("the arrived UE was never indexed by its SUPI, so no NAS message can reach it")
	}

	if state := ue.State(); state != amf.Registered {
		t.Errorf("UE state = %s, want Registered", state)
	}

	if len(smfSbi.N2HandoverCompleteCalls) != 1 {
		t.Errorf("SMF handover completions = %d, want 1", len(smfSbi.N2HandoverCompleteCalls))
	}

	if got := peer.relocationsCompleted(); len(got) != 1 || got[0] != arrivingRelocationID {
		t.Errorf("the MME was told about relocations %v, want [%d]", got, arrivingRelocationID)
	}

	if amfInstance.HandoverInProgress(ue) {
		t.Error("the completed handover was left staged")
	}

	if ebi, ok := ue.EPSBearerIdentity(arrivingPDUSessionID); !ok || ebi != arrivingEPSBearerID {
		t.Errorf("EPS bearer identity = %d/%t, want %d: a later handover to EPS needs it", ebi, ok, arrivingEPSBearerID)
	}
}

func TestHandoverFailureFromEPSRefusesTheRelocation(t *testing.T) {
	peer := &epsPeerStub{}
	amfInstance, radio, sender, _ := arrivingAMF(t, peer)

	done := forwardArrival(t, amfInstance, arrivingRequest())
	hoReq := sender.awaitHandoverRequest(t)

	HandleHandoverFailure(context.Background(), amfInstance, radio, &ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(hoReq.AMFUENGAPID),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioResourcesNotAvailable},
	})

	if _, err := awaitArrival(t, done); !errors.Is(err, interworking.ErrTargetRefused) {
		t.Fatalf("error = %v, want ErrTargetRefused", err)
	}
}

func TestForwardRelocationRefusesAnUnservedTrackingArea(t *testing.T) {
	amfInstance := arrivingAMFOnly(t)

	req := arrivingRequest()
	req.Target.SelectedTAI.TAC = 0xbeef

	if _, err := amfInstance.ForwardRelocation(context.Background(), req); !errors.Is(err, interworking.ErrUnknownTarget) {
		t.Fatalf("error = %v, want ErrUnknownTarget", err)
	}
}

func TestForwardRelocationRefusesAnUnconnectedTarget(t *testing.T) {
	amfInstance := arrivingAMFOnly(t)

	req := arrivingRequest()
	req.Target.ID = 0x0fedcb

	if _, err := amfInstance.ForwardRelocation(context.Background(), req); !errors.Is(err, interworking.ErrUnknownTarget) {
		t.Fatalf("error = %v, want ErrUnknownTarget", err)
	}
}

func TestRelocationCancelFromEPSUnwindsTheArrival(t *testing.T) {
	peer := &epsPeerStub{}
	amfInstance, _, sender, smfSbi := arrivingAMF(t, peer)

	req := arrivingRequest()
	done := forwardArrival(t, amfInstance, req)
	sender.awaitHandoverRequest(t)

	if err := amfInstance.RelocationCancel(context.Background(), req.SUPI, arrivingRelocationID); err != nil {
		t.Fatalf("RelocationCancel: %v", err)
	}

	if _, err := awaitArrival(t, done); err == nil {
		t.Fatal("a cancelled arrival was reported as prepared")
	}

	if _, ok := amfInstance.LookupUeBySupi(req.SUPI); ok {
		t.Error("a cancelled arrival left a UE context behind")
	}

	if len(smfSbi.N2HandoverCanceledCalls) == 0 {
		t.Error("the anchor was never told to put the downlink back on the source eNB")
	}

	if len(smfSbi.ReleaseSmContextCalls) == 0 {
		t.Error("the 5GS half of the arriving session was never released")
	}

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Errorf("UE Context Release Commands to the target = %d, want 1", len(sender.SentUEContextReleaseCommands))
	}

	if err := amfInstance.RelocationCancel(context.Background(), req.SUPI, arrivingRelocationID); err == nil {
		t.Error("a second cancel of the same attempt was accepted")
	}
}

func TestArrivalGuardUnwindsAUEThatNeverShowsUp(t *testing.T) {
	peer := &epsPeerStub{}
	amfInstance, radio, sender, _ := arrivingAMF(t, peer)
	amfInstance.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	req := arrivingRequest()
	done := forwardArrival(t, amfInstance, req)
	hoReq := sender.awaitHandoverRequest(t)

	targetConn := amfInstance.FindUEByAmfUeNgapID(radio, models.AmfUeNgapID(hoReq.AMFUENGAPID))
	if targetConn == nil {
		t.Fatal("the target connection is gone")
	}

	arrivingUe := targetConn.UeContext()

	admitArrival(t, amfInstance, radio, hoReq, arrivingPDUSessionID)

	if _, err := awaitArrival(t, done); err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for amfInstance.HandoverInProgress(arrivingUe) {
		if time.Now().After(deadline) {
			t.Fatal("the arrival outlived its guard")
		}

		time.Sleep(time.Millisecond)
	}

	if _, ok := amfInstance.LookupUeBySupi(req.SUPI); ok {
		t.Error("an abandoned arrival left a UE context behind")
	}
}
