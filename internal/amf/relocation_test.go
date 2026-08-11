// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"go.uber.org/zap"
)

const testRelocationIMSI = "001010000000001"

type fakeEPSPeer struct {
	mu        sync.Mutex
	request   interworking.ForwardRelocationRequest
	response  interworking.ForwardRelocationResponse
	err       error
	cancelled []string
	block     chan struct{}
}

func (f *fakeEPSPeer) ForwardRelocation(ctx context.Context, req interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	f.mu.Lock()
	f.request = req
	block := f.block
	f.mu.Unlock()

	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return interworking.ForwardRelocationResponse{}, ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	return f.response, f.err
}

func (f *fakeEPSPeer) RelocationCancel(_ context.Context, imsi string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cancelled = append(f.cancelled, imsi)

	return nil
}

func (f *fakeEPSPeer) forwarded() interworking.ForwardRelocationRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.request
}

func (f *fakeEPSPeer) cancels() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.cancelled...)
}

func newRelocatingAMF(t *testing.T, peer *fakeEPSPeer) (*amf.AMF, *amf.UeContext, *amf.UeConn) {
	t.Helper()

	a := amf.New(nil, nil, nil)
	a.EPS = peer

	ue := relocatableUE(t)
	ue.SetSupiForTest(mustSUPIFromIMSI(t, testRelocationIMSI))

	source := amf.NewUeConnForTest(newRadioForTest(a, &sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	source.AMFForTest().AttachUeConn(ue, source)

	return a, ue, source
}

func TestPrepareHandoverToEPS(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	prep, err := a.PrepareHandoverToEPS(ue, source, testTarget, []byte{0xde, 0xad}, []uint8{1})
	if err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if !a.HandoverToEPSInProgress(ue) {
		t.Fatal("the handover is not staged as a move to EPS")
	}

	if len(prep.Request.PDNConnections) != 1 || prep.Request.PDNConnections[0].PDUSessionID != 1 {
		t.Fatalf("PDN connections = %+v, want the one session with an EPS bearer identity", prep.Request.PDNConnections)
	}

	// TS 33.501 §8.3.2 step 7
	if want := prep.Request.SecurityContext.DLNASCount; prep.Container.SequenceNumber != uint8(want-1) {
		t.Fatalf("container sequence number = %d, want one below the mapped downlink COUNT %d",
			prep.Container.SequenceNumber, want)
	}
}

func TestPrepareHandoverToEPSOffersOnlyTheRequestedSessions(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	if err := ue.CreateSmContext(2, "ref-2", &models.Snssai{Sst: 1}, "ims"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := ue.AllocateEPSBearerIdentity(2); err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	prep, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{2})
	if err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if len(prep.Request.PDNConnections) != 1 || prep.Request.PDNConnections[0].PDUSessionID != 2 {
		t.Fatalf("PDN connections = %+v, want only the requested session", prep.Request.PDNConnections)
	}
}

func TestPrepareHandoverToEPSWithoutAPeer(t *testing.T) {
	a := amf.New(nil, nil, nil)

	ue := relocatableUE(t)
	ue.SetSupiForTest(mustSUPIFromIMSI(t, testRelocationIMSI))

	if _, err := a.PrepareHandoverToEPS(ue, nil, testTarget, nil, []uint8{1}); !errors.Is(err, amf.ErrNoEPSPeer) {
		t.Fatalf("error = %v, want ErrNoEPSPeer", err)
	}
}

func TestPrepareHandoverToEPSLeavesNothingStagedOnFailure(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	if _, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{9}); !errors.Is(err, amf.ErrNoTransferableSessions) {
		t.Fatalf("error = %v, want ErrNoTransferableSessions", err)
	}

	if a.HandoverInProgress(ue) {
		t.Fatal("a failed preparation left a handover staged")
	}

	if _, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1}); err != nil {
		t.Fatalf("the key chain was not released: %v", err)
	}
}

func TestPrepareHandoverToEPSRefusesASecondHandover(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	if _, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1}); err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if _, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1}); !errors.Is(err, amf.ErrRelocationRefused) {
		t.Fatalf("error = %v, want ErrRelocationRefused", err)
	}
}

func TestForwardRelocationReachesThePeer(t *testing.T) {
	peer := &fakeEPSPeer{response: interworking.ForwardRelocationResponse{
		TargetToSource:      []byte{0x01},
		AcceptedPDUSessions: []uint8{1},
	}}
	a, ue, source := newRelocatingAMF(t, peer)

	prep, err := a.PrepareHandoverToEPS(ue, source, testTarget, []byte{0xaa}, []uint8{1})
	if err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	resp, err := a.ForwardRelocation(context.Background(), prep.Request)
	if err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	if len(resp.AcceptedPDUSessions) != 1 {
		t.Fatalf("accepted PDU sessions = %v", resp.AcceptedPDUSessions)
	}

	if got := peer.forwarded(); got.IMSI != ue.Supi().IMSI() || got.Target != testTarget {
		t.Fatalf("the peer received %+v", got)
	}
}

func TestForwardRelocationIsBounded(t *testing.T) {
	peer := &fakeEPSPeer{block: make(chan struct{})}
	a, ue, source := newRelocatingAMF(t, peer)
	a.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	prep, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1})
	if err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if _, err := a.ForwardRelocation(context.Background(), prep.Request); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want a deadline", err)
	}

	close(peer.block)
}

func TestCancelHandoverToEPSReachesThePeer(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	if _, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1}); err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	target, toEPS, aborted := a.CancelHandover(ue)
	if !aborted || !toEPS || target != nil {
		t.Fatalf("CancelHandover = (%v, %t, %t), want an aborted move to EPS with no target gNB", target, toEPS, aborted)
	}

	a.CancelRelocationToEPS(context.Background(), ue)

	if got := peer.cancels(); len(got) != 1 || got[0] != ue.Supi().IMSI() {
		t.Fatalf("the peer was told to cancel %v", got)
	}
}

func TestRelocationCompleteReleasesTheFiveGSSide(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	if err := a.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	if _, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1}); err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if err := a.RelocationComplete(context.Background(), ue.Supi().IMSI()); err != nil {
		t.Fatalf("RelocationComplete: %v", err)
	}

	if len(ue.SmContextRefs()) != 0 {
		t.Error("the UE still holds SM contexts after moving to EPS")
	}

	if len(ue.EPSBearerIdentities()) != 0 {
		t.Error("the UE still holds EPS bearer identity allocations after moving to EPS")
	}

	if a.HandoverInProgress(ue) {
		t.Error("the handover FSM outlived the move")
	}
}

func TestRelocationCompleteForAUEThatIsNotMovingToEPS(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, _ := newRelocatingAMF(t, peer)

	if err := a.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	if err := a.RelocationComplete(context.Background(), ue.Supi().IMSI()); !errors.Is(err, amf.ErrRelocationNotToEPS) {
		t.Fatalf("error = %v, want ErrRelocationNotToEPS", err)
	}

	if err := a.RelocationComplete(context.Background(), "001010000000009"); !errors.Is(err, amf.ErrNoRelocatingUe) {
		t.Fatalf("error = %v, want ErrNoRelocatingUe", err)
	}
}
