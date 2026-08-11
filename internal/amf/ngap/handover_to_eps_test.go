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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

const relocationTargetENBID = 0x00abc

type epsPeerStub struct {
	mu sync.Mutex

	accepted  []uint8
	err       error
	request   *interworking.ForwardRelocationRequest
	cancelled int
}

func (p *epsPeerStub) ForwardRelocation(_ context.Context, req interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.request = &req

	if p.err != nil {
		return interworking.ForwardRelocationResponse{}, p.err
	}

	return interworking.ForwardRelocationResponse{
		TargetToSource:      []byte{0x0a, 0x0b},
		AcceptedPDUSessions: p.accepted,
	}, nil
}

func (p *epsPeerStub) RelocationCancel(_ context.Context, _ etsi.SUPI) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cancelled++

	return nil
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

func relocatingUe(t *testing.T, peer *epsPeerStub, pduSessionIDs ...uint8) (*amf.AMF, *amf.UeContext, *relocationSignalSender, *amf.Radio) {
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

		if _, err := amfUe.AllocateEPSBearerIdentity(pduSessionID); err != nil {
			t.Fatalf("AllocateEPSBearerIdentity: %v", err)
		}
	}

	sender := newRelocationSender()
	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: sender}

	amfInstance := amf.New(&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, &fakeSmfSbi{SMF: smfInstance})
	amfInstance.EPS = peer

	sourceRan.BindAMFForTest(amfInstance)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 1, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	return amfInstance, amfUe, sender, sourceRan
}

type relocationSignalSender struct {
	*fakeNGAPSender
	outcome chan struct{}
	once    sync.Once
}

func (s *relocationSignalSender) WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error) {
	n, err := s.fakeNGAPSender.WriteMsg(b, info)

	if len(s.SentHandoverCommands) > 0 || len(s.SentHandoverPreparationFailures) > 0 {
		s.once.Do(func() { close(s.outcome) })
	}

	return n, err
}

func newRelocationSender() *relocationSignalSender {
	return &relocationSignalSender{fakeNGAPSender: &fakeNGAPSender{}, outcome: make(chan struct{})}
}

func awaitCommand(t *testing.T, sender *relocationSignalSender) {
	t.Helper()

	select {
	case <-sender.outcome:
	case <-time.After(2 * time.Second):
		t.Fatal("the source gNB got neither a Handover Command nor a Preparation Failure")
	}
}

func TestHandoverRequiredToEPS(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	awaitCommand(t, sender)

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

	// TS 33.501 §8.3.2 step 2, §8.6.1
	if want := req.SecurityContext.DLNASCount; container.SequenceNumber != uint8(want-1) {
		t.Errorf("container sequence number = %d, want one below the mapped downlink COUNT %d",
			container.SequenceNumber, want)
	}

	if string(cmd.TargetToSourceTransparentContainer) != string([]byte{0x0a, 0x0b}) {
		t.Errorf("target-to-source container = % x", cmd.TargetToSourceTransparentContainer)
	}

	// No data forwarding: the list would only name forwarding endpoints.
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

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1, 2))
	awaitCommand(t, sender)

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

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	awaitCommand(t, sender)

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

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	awaitCommand(t, sender)

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

func TestRelocationCompleteReleasesTheSourceGNB(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)

	if err := amfInstance.CommitUEIdentity(context.Background(), amfUe, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	awaitCommand(t, sender)

	if err := amfInstance.RelocationComplete(context.Background(), amfUe.Supi()); err != nil {
		t.Fatalf("RelocationComplete: %v", err)
	}

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("got %d UE Context Release Commands to the source gNB, want 1", len(sender.SentUEContextReleaseCommands))
	}

	if len(amfUe.SmContextRefs()) != 0 {
		t.Error("the UE still holds SM contexts after moving to EPS")
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

func TestHandoverToEPSGuardReleasesAUEThatNeverArrives(t *testing.T) {
	peer := &epsPeerStub{accepted: []uint8{1}}
	amfInstance, amfUe, sender, sourceRan := relocatingUe(t, peer, 1)
	amfInstance.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	HandleHandoverRequired(context.Background(), amfInstance, sourceRan, handoverRequiredToENB(t, 1))
	awaitCommand(t, sender)

	deadline := time.Now().Add(2 * time.Second)
	for amfInstance.HandoverInProgress(amfUe) {
		if time.Now().After(deadline) {
			t.Fatal("the handover outlived its guard")
		}

		time.Sleep(time.Millisecond)
	}

	peer.mu.Lock()
	cancelled := peer.cancelled
	peer.mu.Unlock()

	if cancelled != 1 {
		t.Errorf("the peer was told to cancel %d times, want 1", cancelled)
	}

	if !amfUe.BeginKeyChainProc(procedure.N2Handover) {
		t.Error("the abandoned handover still holds the UE's key chain")
	}
}
