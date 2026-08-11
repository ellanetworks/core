// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"go.uber.org/zap"
)

const testRelocationIMSI = "001010000000001"

type fakeEPSPeer struct {
	mu           sync.Mutex
	request      interworking.ForwardRelocationRequest
	response     interworking.ForwardRelocationResponse
	err          error
	cancelled    []string
	cancelledIDs []interworking.RelocationID
	block        chan struct{}
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

func (f *fakeEPSPeer) RelocationCancel(_ context.Context, supi etsi.SUPI, id interworking.RelocationID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cancelled = append(f.cancelled, supi.IMSI())
	f.cancelledIDs = append(f.cancelledIDs, id)

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

	if got := peer.forwarded(); got.SUPI != ue.Supi() || got.Target != testTarget {
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

	prep, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1})
	if err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if err := a.CancelRelocationToEPS(context.Background(), ue, prep.Request.ID); err != nil {
		t.Fatalf("CancelRelocationToEPS: %v", err)
	}

	if got := peer.cancels(); len(got) != 1 || got[0] != ue.Supi().IMSI() {
		t.Fatalf("the peer was told to cancel %v", got)
	}

	target, aborted := a.CancelHandover(ue)
	if !aborted || target != nil {
		t.Fatalf("CancelHandover = (%v, %t), want an aborted handover with no target gNB", target, aborted)
	}
}

func TestRelocationCompleteReleasesTheFiveGSSide(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	if err := a.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	prep, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1})
	if err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if err := a.RelocationComplete(context.Background(), ue.Supi(), prep.Request.ID); err != nil {
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

// A source gNB that cancels one attempt is free to start another as soon as it is
// acknowledged, while the goroutine that prepared the first is still unwinding.
// That late unwinding must leave the newer attempt — local state included —
// untouched.
func TestAbandoningOneAttemptSparesTheNextOne(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, source := newRelocatingAMF(t, peer)

	first, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1})
	if err != nil {
		t.Fatalf("PrepareHandoverToEPS: %v", err)
	}

	if target, aborted := a.CancelHandover(ue); !aborted || target != nil {
		t.Fatalf("CancelHandover = (%v, %t), want the first attempt aborted", target, aborted)
	}

	second, err := a.PrepareHandoverToEPS(ue, source, testTarget, nil, []uint8{1})
	if err != nil {
		t.Fatalf("the second PrepareHandoverToEPS: %v", err)
	}

	if second.Request.ID == first.Request.ID {
		t.Fatal("the two attempts share an identifier")
	}

	a.AbandonHandoverToEPS(context.Background(), ue, first.Request.ID)

	held, ok := a.RelocationToEPS(ue)
	if !ok || held != second.Request.ID {
		t.Fatalf("abandoning attempt %d unwound the AMF's attempt %d (holds %d, %t)",
			first.Request.ID, second.Request.ID, held, ok)
	}

	if got := peer.cancels(); len(got) != 0 {
		t.Fatalf("the peer was told to cancel %v for an attempt it was never asked about", got)
	}
}

func TestRelocationCompleteForAUEThatIsNotMovingToEPS(t *testing.T) {
	peer := &fakeEPSPeer{}
	a, ue, _ := newRelocatingAMF(t, peer)

	if err := a.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	if err := a.RelocationComplete(context.Background(), ue.Supi(), 1); !errors.Is(err, amf.ErrRelocationNotToEPS) {
		t.Fatalf("error = %v, want ErrRelocationNotToEPS", err)
	}

	if err := a.RelocationComplete(context.Background(), mustSUPIFromIMSI(t, "001010000000009"), 1); !errors.Is(err, amf.ErrNoRelocatingUe) {
		t.Fatalf("error = %v, want ErrNoRelocatingUe", err)
	}
}
