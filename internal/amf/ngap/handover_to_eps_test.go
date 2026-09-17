// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
)

const relocationTargetENBID = 0x00abc

func (*epsPeerStub) CancelRegistration(context.Context, etsi.SUPI) {}

type epsPeerStub struct {
	mu sync.Mutex

	accepted   []uint8
	err        error
	cancelErr  error
	request    *interworking.ForwardRelocationRequest
	requests   int
	cancelled  int
	completed  []interworking.RelocationID
	gate       chan struct{}
	cancelGate chan struct{}
}

func (p *epsPeerStub) MMContext(context.Context, interworking.MMContextRequest) (interworking.MMContextResponse, error) {
	return interworking.MMContextResponse{}, interworking.ErrUnknownUEContext
}

func (p *epsPeerStub) MMContextAck(context.Context, etsi.SUPI, []uint8) error { return nil }

func (p *epsPeerStub) ForwardRelocation(_ context.Context, req interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	p.mu.Lock()
	p.request = &req
	p.requests++
	gate := p.gate
	p.mu.Unlock()

	if gate != nil {
		<-gate
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.err != nil {
		return interworking.ForwardRelocationResponse{}, p.err
	}

	return interworking.ForwardRelocationResponse{
		TargetToSource:      []byte{0x0a, 0x0b},
		AcceptedPDUSessions: p.accepted,
	}, nil
}

func (p *epsPeerStub) forwards() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.requests
}

func (p *epsPeerStub) relocationID() interworking.RelocationID {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.request.ID
}

func (p *epsPeerStub) RelocationComplete(_ context.Context, _ etsi.SUPI, id interworking.RelocationID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.completed = append(p.completed, id)

	return nil
}

func (p *epsPeerStub) relocationsCompleted() []interworking.RelocationID {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]interworking.RelocationID(nil), p.completed...)
}

func (p *epsPeerStub) RelocationCancel(_ context.Context, _ etsi.SUPI, _ interworking.RelocationID) error {
	p.mu.Lock()
	p.cancelled++
	gate, err := p.cancelGate, p.cancelErr
	p.mu.Unlock()

	if gate != nil {
		<-gate
	}

	return err
}

func (p *epsPeerStub) cancels() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.cancelled
}

func (p *epsPeerStub) forwarded() *interworking.ForwardRelocationRequest {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.request
}

func handoverRequiredToENB(t *testing.T, sessions ...uint8) *ngap.HandoverRequired {
	t.Helper()

	msg := handoverRequired(t, 1, sessions...)
	msg.HandoverType = ngap.HandoverTypeFiveGSToEPS

	msg.TargetID = ngap.TargetID{TargeteNBID: &ngap.TargeteNBID{
		GlobalENBID: ngap.GlobalNgENBID{
			PLMNIdentity: operatorPLMN,
			NgENBID: ngap.NgENBID{
				Kind:  ngap.NgENBIDMacro,
				Value: relocationTargetENBID,
			},
		},
		SelectedEPSTAI: ngap.EPSTAI{PLMNIdentity: operatorPLMN, TAC: 1},
	}}

	return msg
}

func relocatingUe(t *testing.T, peer *epsPeerStub, pduSessionIDs ...uint8) (*amf.AMF, *amf.UeContext, *fakeNGAPSender, *amf.Radio) {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("supi: %v", err)
	}

	smfInstance := smf.New(nil, nil, nil, nil)

	amfUe := amf.NewUeContext()
	amfUe.SetSupiForTest(supi)
	amfUe.SetSecuredForTest(true)
	amfUe.SetNgKsiForTest(models.NgKsi{Ksi: 1})
	amfUe.SetKamfForTest("6c38fea1e0a2ff9f8ba6a1e4f4de8b8e1b3b7f2e9d5c0a4738261f5e0d9c8b7a")
	amfUe.SetNHForTest(make([]byte, 32))
	amfUe.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0})
	amfUe.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}

	// TS 33.501 §8.3.2
	amfUe.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, []byte{0xe0, 0xe0, 0x00, 0x00})
	amfUe.SetAllow4G(true)

	if _, ok := amfUe.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("selecting the EPS NAS algorithms failed")
	}

	amfUe.MarkEPSNASAlgorithmsDelivered()

	for _, pduSessionID := range pduSessionIDs {
		smCtx, _ := smfInstance.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: pduSessionID}, "internet", &models.Snssai{Sst: 1})
		smCtx.Tunnel = &smf.UPTunnel{N3TEID: 1234, N3IPv4: netip.MustParseAddr("10.0.0.1")}

		if err := amfUe.CreateSmContext(pduSessionID, smCtx.Ref, &models.Snssai{Sst: 1}, "internet"); err != nil {
			t.Fatalf("CreateSmContext: %v", err)
		}

		ebi, err := amfUe.NextEPSBearerIdentity(pduSessionID)
		if err != nil {
			t.Fatalf("NextEPSBearerIdentity: %v", err)
		}

		amfUe.SetEPSBearerIdentity(pduSessionID, ebi)
	}

	sender := newRelocationSender()
	sourceRan := &amf.Radio{Conn: sender}

	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, &fakeSmfSbi{SMF: smfInstance})
	amfInstance.EPS = peer

	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1)
	sourceUe.AMFForTest().AttachUeConn(t.Context(), amfUe, sourceUe)

	return amfInstance, amfUe, sender, sourceRan
}

func newRelocationSender() *fakeNGAPSender {
	return &fakeNGAPSender{}
}

func awaitHandoverToEPS(t *testing.T, amfInstance *amf.AMF) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	if err := amfInstance.AwaitHandoversToEPS(ctx); err != nil {
		t.Fatalf("the handover to EPS never settled: %v", err)
	}
}

func settleHandoverToEPS(t *testing.T, amfInstance *amf.AMF, sender *fakeNGAPSender, want int) {
	t.Helper()

	awaitHandoverToEPS(t, amfInstance)

	if got := sender.count(); got != want {
		t.Fatalf("the source gNB got %d messages, want %d", got, want)
	}
}

func TestHandoverRequiredToEPS(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	if len(sender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("the handover was refused: %+v", sender.SentHandoverPreparationFailures[0])
	}

	if len(sender.SentHandoverCommands) != 1 {
		t.Fatalf("got %d Handover Commands, want 1", len(sender.SentHandoverCommands))
	}

	cmd := sender.SentHandoverCommands[0]

	if cmd.HandoverType != ngap.HandoverTypeFiveGSToEPS {
		t.Errorf("handover type = %d, want fivegs-to-eps", cmd.HandoverType)
	}

	// TS 38.413 §9.2.3.2
	container, err := fgs.ParseN1ModeToS1ModeNASTransparentContainer(cmd.NASSecurityParametersFromNGRAN)
	if err != nil {
		t.Fatalf("NAS security parameters: %v", err)
	}

	req := peer.forwarded()
	if req == nil {
		t.Fatal("the EPS peer was never asked to prepare the handover")
	}

	if want := req.SecurityContext.DLNASCount; container.SequenceNumber != uint8(want-1) {
		t.Errorf("container sequence number = %d, want one below the mapped downlink COUNT %d",
			container.SequenceNumber, want)
	}

	if string(cmd.TargetToSourceTransparentContainer) != string([]byte{0x0a, 0x0b}) {
		t.Errorf("target-to-source container = % x", cmd.TargetToSourceTransparentContainer)
	}

	if cmd.PDUSessionResourceHandoverList != nil {
		t.Errorf("Handover Command names data-forwarding endpoints: %+v", cmd.PDUSessionResourceHandoverList)
	}

	if len(cmd.PDUSessionResourceToReleaseList) != 0 {
		t.Errorf("an accepted session was ordered released: %+v", cmd.PDUSessionResourceToReleaseList)
	}

	if req.Target.ID != 0xabc || req.Target.Bits != 20 {
		t.Errorf("target eNB = %+v", req.Target)
	}

	if len(req.PDNConnections) != 1 || req.PDNConnections[0].PDUSessionID != 1 {
		t.Errorf("PDN connections = %+v", req.PDNConnections)
	}

	if !amfInstance.HandoverToEPSInProgress(amfUe) {
		t.Error("the handover is not held open for the completion notification")
	}
}

// TS 23.502 §4.11.1.2.1 step 12
func TestHandoverRequiredToEPSReleasesUnacceptedSessions(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, _, sender, sourceRan := relocatingUe(t, peer, 1, 2)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1, 2))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	if len(sender.SentHandoverCommands) != 1 {
		t.Fatalf("got %d Handover Commands, want 1", len(sender.SentHandoverCommands))
	}

	toRelease := sender.SentHandoverCommands[0].PDUSessionResourceToReleaseList
	if len(toRelease) != 1 || toRelease[0].PDUSessionID != 2 {
		t.Fatalf("to-release list = %+v, want only PDU session 2", toRelease)
	}
}

func TestHandoverRequiredToEPSPeerRefuses(t *testing.T) {
	peer := &epsPeerStub{err: errors.New("no target eNB")}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	if len(sender.SentHandoverCommands) != 0 {
		t.Fatalf("a refused handover drew a Handover Command")
	}

	if len(sender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("got %d Handover Preparation Failures, want 1", len(sender.SentHandoverPreparationFailures))
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Error("a refused handover was left staged")
	}
}

func TestHandoverRequiredToEPSWithARANNodeTarget(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, _, sender, sourceRan := relocatingUe(t, peer, 1)

	msg := handoverRequired(t, 1, 1)
	msg.HandoverType = ngap.HandoverTypeFiveGSToEPS

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, msg)

	if len(sender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("got %d Handover Preparation Failures, want 1", len(sender.SentHandoverPreparationFailures))
	}

	if peer.forwarded() != nil {
		t.Error("the peer was asked to prepare a handover to a target that is not an eNB")
	}
}

func TestHandoverRequiredToEPSWithNoTransferableSession(t *testing.T) {
	peer := &epsPeerStub{}
	amfInstance, _, sender, sourceRan := relocatingUe(t, peer)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))

	if len(sender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("got %d Handover Preparation Failures, want 1", len(sender.SentHandoverPreparationFailures))
	}

	if peer.forwarded() != nil {
		t.Error("the peer was asked to prepare a handover with no PDN connection")
	}
}

// TS 23.502 §4.11.1.2.3
func TestHandoverCancelToEPS(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	HandleHandoverCancel(context.Background(), amfInstance, sourceRan, &ngap.HandoverCancel{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
	})

	if len(sender.SentHandoverCancelAcknowledges) != 1 {
		t.Fatalf("got %d Handover Cancel Acknowledges, want 1", len(sender.SentHandoverCancelAcknowledges))
	}

	peer.mu.Lock()
	cancelled := peer.cancelled
	peer.mu.Unlock()

	if cancelled != 1 {
		t.Fatalf("the peer was told to cancel %d times, want 1", cancelled)
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Error("the cancelled handover was left staged")
	}
}

func TestHandoverCancelToEPSUnwindsWhenThePeerHoldsNothing(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}, cancelErr: interworking.ErrRelocationTooLate}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	HandleHandoverCancel(context.Background(), amfInstance, sourceRan, &ngap.HandoverCancel{AMFUENGAPID: 1, RANUENGAPID: 1})

	if !amfInstance.HandoverInProgress(amfUe) {
		t.Error("a handover the peer reported as too late to cancel was unwound anyway")
	}

	peer2 := &epsPeerStub{accepted: []uint8{1}, cancelErr: errors.New("no such relocation")}
	amfInstance2, amfUe2, sender2, sourceRan2 := relocatingUe(t, peer2, 1)

	before2 := sender2.count()

	HandleHandoverRequired(context.Background(), amfInstance2, sourceRan2, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance2, sender2, before2+1)

	HandleHandoverCancel(context.Background(), amfInstance2, sourceRan2, &ngap.HandoverCancel{AMFUENGAPID: 1, RANUENGAPID: 1})

	if amfInstance2.HandoverInProgress(amfUe2) {
		t.Error("the AMF kept a handover no peer is holding, wedging the UE until its guard expires")
	}

	if len(sender2.SentHandoverCancelAcknowledges) != 1 {
		t.Errorf("got %d Handover Cancel Acknowledges, want 1", len(sender2.SentHandoverCancelAcknowledges))
	}
}

// TS 38.413 §8.4.1.1
func TestNoPreparationFailureAfterACancelAcknowledge(t *testing.T) {
	peer := &epsPeerStub{gate: make(chan struct{}), err: errors.New("abandoned")}
	amfInstance, _, sender, sourceRan := relocatingUe(t, peer, 1)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))

	HandleHandoverCancel(context.Background(), amfInstance, sourceRan, &ngap.HandoverCancel{AMFUENGAPID: 1, RANUENGAPID: 1})

	if len(sender.SentHandoverCancelAcknowledges) != 1 {
		t.Fatalf("got %d Handover Cancel Acknowledges, want 1", len(sender.SentHandoverCancelAcknowledges))
	}

	close(peer.gate)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(sender.SentHandoverPreparationFailures) != 0 {
			t.Fatalf("the gNB was sent a Handover Preparation Failure for a preparation it had already cancelled")
		}

		time.Sleep(time.Millisecond)
	}
}

func TestRelocationCompleteReleasesTheSourceGNB(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	if err := amfInstance.CommitUEIdentity(context.Background(), amfUe, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	if err := amfInstance.RelocationComplete(context.Background(), amfUe.Supi(), peer.relocationID()); err != nil {
		t.Fatalf("RelocationComplete: %v", err)
	}

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("got %d UE Context Release Commands to the source gNB, want 1", len(sender.SentUEContextReleaseCommands))
	}

	if len(amfUe.SmContextRefs()) != 0 {
		t.Error("the UE still holds SM contexts after moving to EPS")
	}
}

// TS 23.501 §5.17.2.1
func TestHandoverToEPSDeregistersTheUEButKeepsItsFiveGSContext(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	amfUe.ForceStateForTest(amf.Registered)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)

	if err := amfInstance.CommitUEIdentity(context.Background(), amfUe, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	imsi := amfUe.Supi().IMSI()

	if _, ok := amfInstance.ConnectedSubscribers()[imsi]; !ok {
		t.Fatal("the UE is not reported as a connected 5G subscriber before the move")
	}

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	if err := amfInstance.RelocationComplete(context.Background(), amfUe.Supi(), peer.relocationID()); err != nil {
		t.Fatalf("RelocationComplete: %v", err)
	}

	if got := amfUe.State(); got != amf.Deregistered {
		t.Errorf("5GMM state after the move to EPS = %v, want Deregistered", got)
	}

	if _, ok := amfInstance.ConnectedSubscribers()[imsi]; ok {
		t.Error("the UE is still reported as a connected 5G subscriber after handing over to EPS")
	}

	amfID, ranID := ngap.AMFUENGAPID(1), ngap.RANUENGAPID(1)
	HandleUEContextReleaseComplete(context.Background(), amfInstance, sourceRan,
		&ngap.UEContextReleaseComplete{AMFUENGAPID: &amfID, RANUENGAPID: &ranID})

	held, ok := amfInstance.LookupUeBySupi(amfUe.Supi())
	if !ok {
		t.Fatal("the 5G security context was thrown away with the source gNB's connection; a return from EPS now costs a primary authentication")
	}

	if held != amfUe {
		t.Error("the retained context is not the one that moved to EPS")
	}

	if got := held.State(); got != amf.Deregistered {
		t.Errorf("5GMM state of the retained context = %v, want Deregistered", got)
	}

	if _, ok := amfInstance.ConnectedSubscribers()[imsi]; ok {
		t.Error("the retained context is reported as a connected 5G subscriber")
	}

	if held.Conn() != nil {
		t.Error("the retained context still holds a RAN connection")
	}
}

// TS 38.413 §8.4.1.3
func TestHandoverToEPSFailureCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want ngap.Cause
	}{
		{"unknown target", fmt.Errorf("wrapped: %w", interworking.ErrUnknownTarget), causeUnknownTargetID},
		{
			"the target eNB's own cause",
			interworking.TargetRefusal{Cause: s1ap.Cause{
				Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell,
			}},
			ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkNoRadioResourcesInTargetCell},
		},
		{"target refused", fmt.Errorf("wrapped: %w", interworking.ErrTargetRefused), causeHOFailureInTarget},
		{"anything else", errors.New("boom"), causeHOFailureInTarget},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := handoverToEPSFailureCause(tc.err); got != tc.want {
				t.Errorf("cause = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestHandoverToEPSRemapsTheEPSContextAcrossAttempts(t *testing.T) {
	peer := &epsPeerStub{err: errors.New("no target eNB")}
	amfInstance, _, sender, sourceRan := relocatingUe(t, peer, 1)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	first := *peer.forwarded()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+2)

	second := *peer.forwarded()

	if peer.forwards() != 2 {
		t.Fatalf("the peer got %d relocation requests, want 2", peer.forwards())
	}

	if second.SecurityContext.DLNASCount == first.SecurityContext.DLNASCount {
		t.Errorf("the downlink NAS COUNT stayed at %v across two mappings", second.SecurityContext.DLNASCount)
	}

	if second.SecurityContext.KASME == first.SecurityContext.KASME {
		t.Error("the retry shipped the mapped K'ASME the first target already holds")
	}
}

func TestAwaitHandoversToEPSWaitsForTheAttemptToSettle(t *testing.T) {
	gate := make(chan struct{})
	peer := &epsPeerStub{err: errors.New("no target eNB"), gate: gate}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if err := amfInstance.AwaitHandoversToEPS(ctx); err == nil {
		t.Fatal("the drain returned while the peer still held the preparation")
	}

	close(gate)
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	if amfInstance.HandoverInProgress(amfUe) {
		t.Error("the UE still holds a handover to EPS after the drain")
	}
}

func TestHandoverToEPSGuardReleasesAUEThatNeverArrives(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)
	amfInstance.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	deadline := time.Now().Add(2 * time.Second)
	for !amfUe.BeginKeyChainProc(procedure.N2Handover) {
		if time.Now().After(deadline) {
			t.Fatal("the abandoned handover never released the UE's key chain")
		}

		time.Sleep(time.Millisecond)
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Error("the key chain came free with the handover still in progress")
	}

	if cancelled := peer.cancels(); cancelled != 1 {
		t.Errorf("the peer was told to cancel %d times, want 1", cancelled)
	}
}

func TestHandoverToEPSGuardHoldsTheKeyChainUntilThePeerIsTold(t *testing.T) {
	cancelling := make(chan struct{})
	peer := &epsPeerStub{accepted: []uint8{1}, cancelGate: cancelling}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)
	amfInstance.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	before := sender.count()

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	settleHandoverToEPS(t, amfInstance, sender, before+1)

	deadline := time.Now().Add(2 * time.Second)
	for peer.cancels() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the guard never told the peer to cancel")
		}

		time.Sleep(time.Millisecond)
	}

	if amfUe.BeginKeyChainProc(procedure.SecurityMode) {
		t.Error("a security mode command took the key chain while the abandoned handover was still unwinding")
	}

	close(cancelling)

	deadline = time.Now().Add(2 * time.Second)
	for !amfUe.BeginKeyChainProc(procedure.SecurityMode) {
		if time.Now().After(deadline) {
			t.Fatal("the finished abandonment never released the UE's key chain")
		}

		time.Sleep(time.Millisecond)
	}
}
