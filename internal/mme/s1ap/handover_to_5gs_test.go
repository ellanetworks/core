// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

const (
	relocationTargetGNBID     = 0x0abcde
	relocationPDUSessionID    = 3
	relocationTargetToSourceB = 0xa5
)

type fiveGSPeerStub struct {
	mu sync.Mutex

	request   *interworking.FiveGSRelocationRequest
	accepted  []uint8
	err       error
	cancelled int
	cancelErr error
	completed []interworking.RelocationID
	gate      chan struct{}
}

func (p *fiveGSPeerStub) ForwardRelocation(_ context.Context, req interworking.FiveGSRelocationRequest) (interworking.FiveGSRelocationResponse, error) {
	p.mu.Lock()
	p.request = &req
	gate := p.gate
	p.mu.Unlock()

	if gate != nil {
		<-gate
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.err != nil {
		return interworking.FiveGSRelocationResponse{}, p.err
	}

	return interworking.FiveGSRelocationResponse{
		TargetToSource:     []byte{relocationTargetToSourceB},
		AcceptedEPSBearers: p.accepted,
	}, nil
}

func (p *fiveGSPeerStub) RelocationCancel(_ context.Context, _ etsi.SUPI, _ interworking.RelocationID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cancelled++

	return p.cancelErr
}

func (p *fiveGSPeerStub) RelocationComplete(_ context.Context, _ etsi.SUPI, id interworking.RelocationID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.completed = append(p.completed, id)

	return nil
}

func (p *fiveGSPeerStub) forwarded() *interworking.FiveGSRelocationRequest {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.request
}

func (p *fiveGSPeerStub) cancels() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.cancelled
}

func relocatingToFiveGSUE(t *testing.T, m *mme.MME, peer *fiveGSPeerStub) (*mme.UeContext, *captureConn) {
	t.Helper()

	m.FiveGS = peer

	ue, source, _ := handoverUE(t, m)

	p := m.LookupPDN(ue, mme.DefaultERABID)
	p.PDUSessionID = relocationPDUSessionID
	p.Snssai = &models.Snssai{Sst: 1}

	return ue, source
}

func handoverRequiredToGNB(ue *mme.UeContext) *s1ap.HandoverRequired {
	req := sampleHandoverRequired(ue)
	req.HandoverType = s1ap.HandoverTypeEPSToFiveGS
	req.Cause = s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkTimeCriticalHandover})
	req.TargetID = s1ap.TargetID{TargetNgRanNodeID: &s1ap.TargetNgRanNodeID{
		GlobalRANNodeID: s1ap.GlobalRANNodeID{GNB: &s1ap.GNB{GlobalGNBID: s1ap.GlobalGNBID{
			PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
			GNBID:        s1ap.GNBID{Value: relocationTargetGNBID, Bits: 24},
		}}},
		SelectedTAI: s1ap.FiveGSTAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}}

	return req
}

func requireHandoverToFiveGS(t *testing.T, m *mme.MME, ue *mme.UeContext, source *captureConn) {
	t.Helper()

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, handoverRequiredToGNB(ue).Marshal)))
}

func awaitSourceMessage(t *testing.T, source *captureConn, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for source.count() < want {
		if time.Now().After(deadline) {
			t.Fatalf("the source eNB got %d messages, want %d", source.count(), want)
		}

		time.Sleep(time.Millisecond)
	}
}

func lastHandoverCommand(t *testing.T, source *captureConn) *s1ap.HandoverCommand {
	t.Helper()

	pdu, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || pdu.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("last message to the source is %T, want a Handover Command", lastPDU(t, source))
	}

	cmd, err := s1ap.ParseHandoverCommand(pdu.Value)
	if err != nil {
		t.Fatalf("parse Handover Command: %v", err)
	}

	return cmd
}

func lastPreparationFailure(t *testing.T, source *captureConn) *s1ap.HandoverPreparationFailure {
	t.Helper()

	pdu, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok || pdu.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("last message to the source is %T, want a Handover Preparation Failure", lastPDU(t, source))
	}

	fail, err := s1ap.ParseHandoverPreparationFailure(pdu.Value)
	if err != nil {
		t.Fatalf("parse Handover Preparation Failure: %v", err)
	}

	return fail
}

func TestHandoverRequiredToFiveGS(t *testing.T) {
	m := newTestMME(t)
	peer := &fiveGSPeerStub{accepted: []uint8{mme.DefaultERABID}}
	ue, source := relocatingToFiveGSUE(t, m, peer)

	before := source.count()

	requireHandoverToFiveGS(t, m, ue, source)
	awaitSourceMessage(t, source, before+1)

	req := peer.forwarded()
	if req == nil {
		t.Fatal("the 5GS peer was never asked to prepare the handover")
	}

	if req.SUPI.IMSI() != testSubscriber.IMSI {
		t.Errorf("the peer was asked about %s, want %s", req.SUPI, testSubscriber.IMSI)
	}

	if req.Target.Kind != interworking.NGRANNodeGNB || req.Target.ID != relocationTargetGNBID || req.Target.Bits != 24 {
		t.Errorf("target = %+v", req.Target)
	}

	if req.Target.SelectedTAI.TAC != 1 {
		t.Errorf("selected 5GS TAI = %+v", req.Target.SelectedTAI)
	}

	if req.Cause.Value != s1ap.CauseRadioNetworkTimeCriticalHandover {
		t.Errorf("cause = %+v, want the source eNB's own", req.Cause)
	}

	if len(req.PDNConnections) != 1 {
		t.Fatalf("PDN connections = %+v, want 1", req.PDNConnections)
	}

	if c := req.PDNConnections[0]; c.PDUSessionID != relocationPDUSessionID || c.EPSBearerIdentity != mme.DefaultERABID || c.APN != "internet" {
		t.Errorf("PDN connection = %+v", c)
	}

	if req.SecurityContext.NCC != 1 {
		t.Errorf("NCC = %d, want the UE's current 1", req.SecurityContext.NCC)
	}

	if req.SecurityContext.KASME == ([32]byte{}) {
		t.Error("no K_ASME for the target AMF to derive K'AMF from")
	}

	if req.SecurityContext.NH == ([32]byte{}) {
		t.Error("no NH for the target AMF to derive K'AMF from")
	}

	if req.SecurityContext.Algorithms.Integrity == 0 && req.SecurityContext.Algorithms.Ciphering == 0 {
		t.Error("no selected EPS NAS algorithms")
	}

	if string(req.SourceToTarget) != string([]byte{0x01, 0x02, 0x03}) {
		t.Errorf("source-to-target container = % x", req.SourceToTarget)
	}

	cmd := lastHandoverCommand(t, source)

	if cmd.HandoverType != s1ap.HandoverTypeEPSToFiveGS {
		t.Errorf("handover type = %d, want eps-to-5gs", cmd.HandoverType)
	}

	if len(cmd.TargetToSource) != 1 || cmd.TargetToSource[0] != relocationTargetToSourceB {
		t.Errorf("target-to-source container = % x", cmd.TargetToSource)
	}

	if len(cmd.ERABToRelease) != 0 {
		t.Errorf("an accepted bearer was ordered released: %+v", cmd.ERABToRelease)
	}

	if cmd.NASSecurityParametersfromEUTRAN != nil {
		t.Error("NAS security parameters travel only to UTRAN/GERAN; the UE's container comes back inside the target-to-source container")
	}

	if _, held := m.RelocationToFiveGS(ue); !held {
		t.Error("the handover is not held open for the completion notification")
	}
}

func TestHandoverRequiredToFiveGSReleasesUnacceptedPDNs(t *testing.T) {
	m := newTestMME(t)
	peer := &fiveGSPeerStub{}
	ue, source := relocatingToFiveGSUE(t, m, peer)

	before := source.count()

	requireHandoverToFiveGS(t, m, ue, source)
	awaitSourceMessage(t, source, before+1)

	cmd := lastHandoverCommand(t, source)
	if len(cmd.ERABToRelease) != 1 || cmd.ERABToRelease[0].ERABID != s1ap.ERABID(mme.DefaultERABID) {
		t.Fatalf("to-release list = %+v, want the bearer the target did not take", cmd.ERABToRelease)
	}
}

func TestHandoverRequiredToFiveGSPeerRefuses(t *testing.T) {
	m := newTestMME(t)
	peer := &fiveGSPeerStub{err: errors.New("no target gNB")}
	ue, source := relocatingToFiveGSUE(t, m, peer)

	before := source.count()

	requireHandoverToFiveGS(t, m, ue, source)
	awaitSourceMessage(t, source, before+1)

	if got := lastPreparationFailure(t, source); got.Cause == nil {
		t.Error("the refusal reached the source eNB without a cause")
	}

	if _, held := m.RelocationToFiveGS(ue); held {
		t.Error("a refused handover was left staged")
	}
}

func TestHandoverRequiredToFiveGSWithAnENBTarget(t *testing.T) {
	m := newTestMME(t)
	peer := &fiveGSPeerStub{accepted: []uint8{mme.DefaultERABID}}
	ue, source := relocatingToFiveGSUE(t, m, peer)

	req := handoverRequiredToGNB(ue)
	req.TargetID = s1ap.TargetID{TargeteNBID: s1ap.TargeteNBID{GlobalENBID: targetGlobalENBID, SelectedTAI: s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}}}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, req.Marshal)))

	if got := lastPreparationFailure(t, source); got.Cause == nil {
		t.Error("no cause in the preparation failure")
	}

	if peer.forwarded() != nil {
		t.Error("the peer was asked to prepare a handover to a target that is not an NG-RAN node")
	}
}

func TestHandoverRequiredToFiveGSWithNoTransferablePDN(t *testing.T) {
	m := newTestMME(t)
	peer := &fiveGSPeerStub{}

	m.FiveGS = peer

	ue, source, _ := handoverUE(t, m)

	requireHandoverToFiveGS(t, m, ue, source)

	if got := lastPreparationFailure(t, source); got.Cause == nil {
		t.Error("no cause in the preparation failure")
	}

	if peer.forwarded() != nil {
		t.Error("the peer was asked to move a PDN connection with no PDU session identity")
	}
}

func TestRelocationCompleteReleasesTheSourceENB(t *testing.T) {
	m := newTestMME(t)
	peer := &fiveGSPeerStub{accepted: []uint8{mme.DefaultERABID}}
	ue, source := relocatingToFiveGSUE(t, m, peer)

	before := source.count()

	requireHandoverToFiveGS(t, m, ue, source)
	awaitSourceMessage(t, source, before+1)

	id, held := m.RelocationToFiveGS(ue)
	if !held {
		t.Fatal("the handover was not staged")
	}

	if err := m.RelocationComplete(context.Background(), ue.Supi(), id); err != nil {
		t.Fatalf("RelocationComplete: %v", err)
	}

	release, ok := lastPDU(t, source).(*s1ap.InitiatingMessage)
	if !ok || release.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("last message to the source is %T, want a UE Context Release Command", lastPDU(t, source))
	}

	if ue.PDNCount() != 0 {
		t.Error("the UE still holds PDN connections after moving to 5GS")
	}

	if _, still := m.LookupUeBySupi(ue.Supi()); still {
		t.Error("the source MME kept the UE context after the handover completed")
	}

	if err := m.RelocationComplete(context.Background(), ue.Supi(), id); err == nil {
		t.Error("a second completion of the same attempt was accepted")
	}
}

func TestHandoverToFiveGSGuardCancelsAUEThatNeverArrives(t *testing.T) {
	m := newTestMME(t)
	m.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	peer := &fiveGSPeerStub{accepted: []uint8{mme.DefaultERABID}}
	ue, source := relocatingToFiveGSUE(t, m, peer)

	before := source.count()

	requireHandoverToFiveGS(t, m, ue, source)
	awaitSourceMessage(t, source, before+1)

	deadline := time.Now().Add(2 * time.Second)

	for {
		if _, held := m.RelocationToFiveGS(ue); !held {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("the handover outlived its guard")
		}

		time.Sleep(time.Millisecond)
	}

	if peer.cancels() != 1 {
		t.Errorf("the peer was told to cancel %d times, want 1", peer.cancels())
	}
}

func TestRelocationCompleteReleasesEvenWhenTheGuardFiredFirst(t *testing.T) {
	m := newTestMME(t)
	peer := &fiveGSPeerStub{accepted: []uint8{mme.DefaultERABID}}
	ue, source := relocatingToFiveGSUE(t, m, peer)

	before := source.count()

	requireHandoverToFiveGS(t, m, ue, source)
	awaitSourceMessage(t, source, before+1)

	id, held := m.RelocationToFiveGS(ue)
	if !held {
		t.Fatal("the handover was not staged")
	}

	if !m.AbandonHandoverToFiveGS(context.Background(), ue, id) {
		t.Fatal("the handover could not be abandoned")
	}

	if err := m.RelocationComplete(context.Background(), ue.Supi(), id); err != nil {
		t.Fatalf("RelocationComplete: %v", err)
	}

	if ue.PDNCount() != 0 {
		t.Error("the UE kept its PDN connections after arriving on 5GS")
	}

	if _, still := m.LookupUeBySupi(ue.Supi()); still {
		t.Error("the source MME kept the UE context after the handover completed")
	}
}
