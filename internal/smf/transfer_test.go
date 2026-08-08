// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

func interworkingFakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF, *fakeMME) {
	pcf, store, upf, amfCb := defaultFakes()
	store.allocatedIP = netip.AddrFrom4([4]byte{10, 45, 0, 7})
	store.releasedIP = store.allocatedIP

	return pcf, store, upf, amfCb, &fakeMME{}
}

func epsMove(pduSessionID uint8) models.EPSBearerRequest {
	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = pduSessionID
	req.RequestType = eps.RequestTypeHandover

	return req
}

const movedPDUSessionID uint8 = 3

func establish5GS(t *testing.T, s *smf.SMF) *smf.SMContext {
	t.Helper()

	ctx := context.Background()

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if reject != nil {
		t.Fatalf("CreateSmContext rejected the establishment: %d-byte N1 reject", len(reject))
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	sc := s.GetSession(ref)
	if sc == nil {
		t.Fatal("5G session is not in the pool")
	}

	return sc
}

type sessionInvariants struct {
	ipv4   string
	seid   uint64
	n3TEID uint32
}

func invariantsOf(t *testing.T, sc *smf.SMContext) sessionInvariants {
	t.Helper()

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.PFCPContext == nil || sc.Tunnel == nil {
		t.Fatal("session has no user plane")
	}

	return sessionInvariants{
		ipv4:   sc.PDUIPV4Address.String(),
		seid:   sc.PFCPContext.SEID,
		n3TEID: sc.Tunnel.N3TEID,
	}
}

func TestTransfer5GSToEPSKeepsTheSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establish5GS(t, s)
	before := invariantsOf(t, sc)

	establishes := len(store.ops())

	bearer, err := s.CreateEPSSession(ctx, epsMove(3))
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	if bearer.Ref != sc.Ref {
		t.Errorf("move returned ref %q, want the session's own %q", bearer.Ref, sc.Ref)
	}

	if got := bearer.IPv4.String(); got != before.ipv4 {
		t.Errorf("UE address after the move = %s, want %s", got, before.ipv4)
	}

	sc.Mutex.Lock()
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access5G {
		t.Error("the session left 5GS before the eNB bound its downlink")
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	after := invariantsOf(t, sc)
	if after != before {
		t.Errorf("session invariants changed across the move: %+v, want %+v", after, before)
	}

	sc.Mutex.Lock()
	access, ebi := sc.Access, sc.EBI
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Error("the session is not on EPS after the eNB bound its downlink")
	}

	if ebi != epsTestEBI {
		t.Errorf("session EBI = %d, want the %d the MME allocated", ebi, epsTestEBI)
	}

	if s.SessionCount() != 1 {
		t.Errorf("sessions = %d, want 1: the move must not create a second", s.SessionCount())
	}

	if got := len(store.ops()); got != establishes {
		t.Errorf("IP pool operations = %d, want %d: the move must not touch the lease", got, establishes)
	}

	calls := amfCb.dropped()
	if len(calls) != 1 {
		t.Fatalf("AMF SessionDropped calls = %d, want 1", len(calls))
	}

	if calls[0].ref != sc.Ref || calls[0].pduSessionID != 3 {
		t.Errorf("AMF told about ref %q pdu session %d, want %q / 3", calls[0].ref, calls[0].pduSessionID, sc.Ref)
	}

	if calls[0].n2Transfer == nil {
		t.Error("no N2 release for the radio resources the moved session held at the gNB")
	}

	if s.GetSession(sc.Ref) == nil {
		t.Error("the session was released by the move")
	}
}

func TestTransferEPSTo5GSKeepsTheSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = 3
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	before := invariantsOf(t, sc)
	establishes := len(store.ops())

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), 3, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("move to 5GS: %v", err)
	}

	if reject != nil {
		t.Fatalf("the move was rejected: %d-byte N1 reject", len(reject))
	}

	if ref != sc.Ref {
		t.Errorf("move returned ref %q, want the session's own %q", ref, sc.Ref)
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	if after := invariantsOf(t, sc); after != before {
		t.Errorf("session invariants changed across the move: %+v, want %+v", after, before)
	}

	sc.Mutex.Lock()
	access, ebi := sc.Access, sc.EBI
	sc.Mutex.Unlock()

	if access != smf.Access5G {
		t.Error("the session is not on 5GS after the gNB bound its downlink")
	}

	if ebi != 0 {
		t.Errorf("session still holds EPS bearer identity %d after moving to 5GS", ebi)
	}

	if s.SessionCount() != 1 {
		t.Errorf("sessions = %d, want 1", s.SessionCount())
	}

	if got := len(store.ops()); got != establishes {
		t.Errorf("IP pool operations = %d, want %d: the move must not touch the lease", got, establishes)
	}

	calls := mmeCb.dropped()
	if len(calls) != 1 {
		t.Fatalf("MME SessionDropped calls = %d, want 1", len(calls))
	}

	if calls[0].ref != sc.Ref || calls[0].ebi != epsTestEBI {
		t.Errorf("MME told about ref %q ebi %d, want %q / %d", calls[0].ref, calls[0].ebi, sc.Ref, epsTestEBI)
	}

	if s.GetSession(sc.Ref) == nil {
		t.Error("the session was released by the move")
	}
}

func TestTransferRefusals(t *testing.T) {
	t.Run("EPS: no such PDU session", func(t *testing.T) {
		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		_, err := s.CreateEPSSession(context.Background(), epsMove(3))
		if err == nil {
			t.Fatal("a move of a session the anchor does not hold succeeded")
		}

		if !isNotTransferable(err) {
			t.Errorf("error = %v, want it to report the session does not exist", err)
		}
	})

	t.Run("EPS: the UE named no identity", func(t *testing.T) {
		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		establish5GS(t, s)

		if _, err := s.CreateEPSSession(context.Background(), epsMove(0)); !isNotTransferable(err) {
			t.Errorf("error = %v, want it to report the session does not exist", err)
		}
	})

	t.Run("EPS: the session is on another data network", func(t *testing.T) {
		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		establish5GS(t, s)

		req := epsMove(3)
		req.APN = "ims"

		err := mustFail(t, s, req)
		if !errors.Is(err, models.ErrSessionOnOtherDNN) {
			t.Errorf("error = %v, want it to report another data network", err)
		}
	})

	t.Run("5GS: no such PDU session", func(t *testing.T) {
		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		ref, reject, err := s.CreateSmContext(context.Background(), testSUPI(), 3, testDNN, testSnssai,
			fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
		if err == nil {
			t.Fatal("a move of a session the anchor does not hold succeeded")
		}

		if ref != "" {
			t.Errorf("a refused move returned ref %q", ref)
		}

		if reject == nil {
			t.Fatal("a refused move produced no reject for the UE")
		}

		assertGSMCause(t, reject, fgs.GSMCausePDUSessionDoesNotExist)
	})

	t.Run("5GS: the session is already on 5GS", func(t *testing.T) {
		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		establish5GS(t, s)

		_, reject, err := s.CreateSmContext(context.Background(), testSUPI(), 3, testDNN, testSnssai,
			fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
		if err == nil {
			t.Fatal("a move onto the access already serving the session succeeded")
		}

		assertGSMCause(t, reject, fgs.GSMCauseInsufficientResources)
	})
}

func TestTransferTo5GSChecksTheSlice(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = 3
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	other := &models.Snssai{Sst: 2, Sd: "0a0b0c"}

	_, reject, err := s.CreateSmContext(ctx, testSUPI(), 3, testDNN, other,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err == nil {
		t.Fatal("a move naming another slice succeeded")
	}

	assertGSMCause(t, reject, fgs.GSMCauseInsufficientResources)

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("the refused move released the session")
	}

	sc.Mutex.Lock()
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Error("the refused move left the session on the wrong access")
	}
}

func TestOneTransferAtATime(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	establish5GS(t, s)

	if _, err := s.CreateEPSSession(ctx, epsMove(3)); err != nil {
		t.Fatalf("first move: %v", err)
	}

	second := epsMove(3)
	second.EPSBearerIdentity = epsTestEBI + 1

	if _, err := s.CreateEPSSession(ctx, second); err == nil {
		t.Fatal("a second move of the same session was admitted while the first was in flight")
	}
}

func mustFail(t *testing.T, s *smf.SMF, req models.EPSBearerRequest) error {
	t.Helper()

	_, err := s.CreateEPSSession(context.Background(), req)
	if err == nil {
		t.Fatal("expected the move to be refused")
	}

	return err
}

func isNotTransferable(err error) bool { return errors.Is(err, models.ErrSessionNotTransferable) }

func assertGSMCause(t *testing.T, reject []byte, want fgs.GSMCause) {
	t.Helper()

	if reject == nil {
		t.Fatal("no reject produced for the UE")
	}

	msg, err := fgs.ParseMessage(reject)
	if err != nil {
		t.Fatalf("parse reject: %v", err)
	}

	got, ok := msg.(*fgs.PDUSessionEstablishmentReject)
	if !ok {
		t.Fatalf("reject is a %T, want a PDU SESSION ESTABLISHMENT REJECT", msg)
	}

	if got.Cause != want {
		t.Errorf("reject cause = %s, want %s", got.Cause, want)
	}
}

// TS 23.501 §5.7.1.1: N3 carries the QFI in the PDU Session Container; S1-U has
// no such header. The downlink FAR must therefore be stamped for the access the
// session is moving *to*, not the one it is leaving.
func TestTransferEPSTo5GSStampsTheDownlinkForN3(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	var ohc *models.OuterHeaderCreation

	for _, call := range upf.modifyCalls {
		for _, far := range call.UpdateFARs {
			if far.ForwardingParameters != nil && far.ForwardingParameters.OuterHeaderCreation != nil {
				ohc = far.ForwardingParameters.OuterHeaderCreation
			}
		}
	}

	if ohc == nil {
		t.Fatal("no downlink FAR with an outer header creation was pushed to the UPF")
	}

	if ohc.TEID != 0x7001 {
		t.Errorf("downlink FAR TEID = %#x, want the gNB's %#x", ohc.TEID, 0x7001)
	}

	if ohc.S1U {
		t.Error("downlink FAR is stamped S1-U after the move to 5GS: N3 would carry no PDU Session Container, so the gNB gets no QFI")
	}
}

// The MME still holds the ref, DefaultEBI and a live E-RAB until the move
// commits, so a failed accept must leave the session where it is rather than
// release it out from under EPS.
func TestTransferEPSTo5GSKeepsTheSessionWhenTheAcceptCannotBeDelivered(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	releasesBefore := len(store.releasedIPs)

	amfCb.mu.Lock()
	amfCb.err = errors.New("no N1N2 path to the UE")
	amfCb.mu.Unlock()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err == nil {
		t.Fatal("the move reported success though the accept could not be delivered")
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("the session the MME still owns was released when the accept failed")
	}

	sc.Mutex.Lock()
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Errorf("session is on %v, want it left on EPS", access)
	}

	if got := len(store.releasedIPs); got != releasesBefore {
		t.Errorf("IP lease releases = %d, want %d: the UE's address was freed while EPS still serves it", got, releasesBefore)
	}

	// The transfer must also be released, or the session can never move again.
	if _, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID)); err == nil {
		t.Fatal("a move to EPS succeeded for a session already on EPS")
	}
}

// A move that cannot commit must leave the session movable. Wedging it with a
// pending move no message can clear means the UE can never return to the access
// it came from.
func TestFailedCommitLeavesTheSessionMovable(t *testing.T) {
	t.Run("the UPF refuses the bind", func(t *testing.T) {
		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		ctx := context.Background()
		sc := establish5GS(t, s)

		if _, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID)); err != nil {
			t.Fatalf("move to EPS: %v", err)
		}

		upf.mu.Lock()
		upf.err = errors.New("UPF refused the modification")
		upf.mu.Unlock()

		enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
		if err := s.ModifyEPSSession(ctx, sc.Ref, enb); err == nil {
			t.Fatal("the bind reported success though the UPF refused it")
		}

		assertMovable(t, sc)
	})

	t.Run("the RAN has no resources", func(t *testing.T) {
		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		ctx := context.Background()

		req := epsRequest(1)
		req.APN = testDNN
		req.PDUSessionID = movedPDUSessionID
		req.Snssai = testSnssai

		bearer, err := s.CreateEPSSession(ctx, req)
		if err != nil {
			t.Fatalf("CreateEPSSession: %v", err)
		}

		enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
		if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
			t.Fatalf("ModifyEPSSession: %v", err)
		}

		if _, _, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
			fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil {
			t.Fatalf("move to 5GS: %v", err)
		}

		fail, err := buildPDUSessionResourceSetupUnsuccessfulTransfer()
		if err != nil {
			t.Fatalf("build N2 failure: %v", err)
		}

		_ = s.UpdateSmContextN2InfoPduResSetupFail(ctx, bearer.Ref, fail)

		assertMovable(t, s.GetSession(bearer.Ref))
	})
}

// assertMovable reports whether a fresh move would be admitted; a wedged session
// refuses every one with "already moving".
func assertMovable(t *testing.T, sc *smf.SMContext) {
	t.Helper()

	if sc == nil {
		t.Fatal("the session was released by a failed move")
	}

	sc.Mutex.Lock()
	moving := sc.TransferPendingForTest()
	sc.Mutex.Unlock()

	if moving {
		t.Fatal("the session is still marked mid-move, so it can never move again")
	}
}

func buildPDUSessionResourceSetupUnsuccessfulTransfer() ([]byte, error) {
	transfer := libngap.PDUSessionResourceSetupUnsuccessfulTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkUnspecified},
	}

	return transfer.Marshal()
}

func TestTransferToEPSChecksTheSlice(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc := establish5GS(t, s)

	move := epsMove(movedPDUSessionID)
	move.Snssai = &models.Snssai{Sst: 2, Sd: "0a0b0c"}

	if _, err := s.CreateEPSSession(context.Background(), move); err == nil {
		t.Fatal("a move onto a policy from another slice succeeded")
	}

	sc.Mutex.Lock()
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access5G {
		t.Error("the refused move took the session off 5GS")
	}
}

func TestTransferTo5GSRefusesEmergencySessions(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	_, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingEmergencyPDUSession, buildPDUSessionEstRequest())
	if err == nil {
		t.Fatal("a move naming an emergency PDU session succeeded")
	}

	assertGSMCause(t, reject, fgs.GSMCausePDUSessionDoesNotExist)

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("the refused move released the session")
	}

	sc.Mutex.Lock()
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Error("the refused move took the session off EPS")
	}
}

func TestTransferFromAnIdle5GSSessionSendsNoN2Release(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establish5GS(t, s)

	if err := s.DeactivateSmContext(ctx, sc.Ref); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	bearer, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID))
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	calls := amfCb.dropped()
	if len(calls) != 1 {
		t.Fatalf("AMF SessionDropped calls = %d, want 1", len(calls))
	}

	if calls[0].n2Transfer != nil {
		t.Error("an N2 release went for a session whose user plane was already down")
	}
}

// The session's policy must describe the access it is on. Between the accept and
// the RAN bind the move has not committed, so a reader must still see the EPS
// policy, not the 5GS one the accept was built from.
func TestTransferCommitsTheTargetPolicyOnlyAtTheRANBind(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	pcf.policy.PolicyID = "5gs-policy"

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai
	req.PolicyID = "eps-policy"

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	sc.Mutex.Lock()
	access, policyID, qfi := sc.Access, sc.PolicyData.PolicyID, sc.PolicyData.QosData.QFI
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Fatal("the session left EPS before the gNB bound its downlink")
	}

	if policyID != "eps-policy" {
		t.Errorf("policy %q is in force on an EPS session, want %q: the target policy was committed early", policyID, "eps-policy")
	}

	if qfi != 0 {
		t.Errorf("QFI %d is in force on an EPS session, want 0", qfi)
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	sc.Mutex.Lock()
	access, policyID, qfi = sc.Access, sc.PolicyData.PolicyID, sc.PolicyData.QosData.QFI
	sc.Mutex.Unlock()

	if access != smf.Access5G {
		t.Fatal("the session is not on 5GS after the gNB bound its downlink")
	}

	if policyID != "5gs-policy" || qfi != 1 {
		t.Errorf("policy %q QFI %d in force after the move, want %q / 1", policyID, qfi, "5gs-policy")
	}
}

// The RA tunnel entry is programmed from the committed policy, not the source
// access's: it already reads the committed access for its encapsulation, so a
// policy read on the other side of the commit would disagree with it.
func TestTransferRegistersTheRAEntryWithTheTargetQFI(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	store.allocatedIPv6 = netip.MustParseAddr("2001:db8:2::")

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(3)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	reg := upf.lastIPv6Reg
	if reg == nil {
		t.Fatal("IPv6 session not registered with the UPF after the move")
	}

	if reg.QFI != 1 {
		t.Errorf("RA entry QFI = %d, want 1: it was programmed from the EPS policy", reg.QFI)
	}

	if reg.S1U {
		t.Error("RA entry marked S1U (PSC-less) for a session now on N3")
	}
}

// The three states a move onto EPS can be released from. Only the first names a
// session 5GS is still serving.
func TestReleaseEPSSessionByTransferState(t *testing.T) {
	prepare := func(t *testing.T) (*smf.SMF, *fakeUPF, *smf.SMContext) {
		t.Helper()

		pcf, store, upf, amfCb, mmeCb := interworkingFakes()
		s := newTestSMF(pcf, store, upf, amfCb)
		s.SetMME(mmeCb)

		sc := establish5GS(t, s)

		if _, err := s.CreateEPSSession(context.Background(), epsMove(movedPDUSessionID)); err != nil {
			t.Fatalf("prepare the move to EPS: %v", err)
		}

		return s, upf, sc
	}

	bind := func(t *testing.T, s *smf.SMF, ref string) error {
		t.Helper()

		enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}

		return s.ModifyEPSSession(context.Background(), ref, enb)
	}

	t.Run("prepared: 5GS is still serving it", func(t *testing.T) {
		s, _, sc := prepare(t)

		if err := s.ReleaseEPSSession(context.Background(), sc.Ref); err != nil {
			t.Fatalf("ReleaseEPSSession: %v", err)
		}

		if s.GetSession(sc.Ref) == nil {
			t.Fatal("the session was released though the move never began committing")
		}

		if _, err := s.CreateEPSSession(context.Background(), epsMove(movedPDUSessionID)); err != nil {
			t.Errorf("the session cannot move again after the abandon: %v", err)
		}
	})

	t.Run("committed: EPS owns it", func(t *testing.T) {
		s, _, sc := prepare(t)

		if err := bind(t, s, sc.Ref); err != nil {
			t.Fatalf("ModifyEPSSession: %v", err)
		}

		if err := s.ReleaseEPSSession(context.Background(), sc.Ref); err != nil {
			t.Fatalf("ReleaseEPSSession: %v", err)
		}

		if s.GetSession(sc.Ref) != nil {
			t.Error("a committed EPS session was not released")
		}
	})

	// The commit takes `pending` before it can fail, so nothing on either side is
	// left marking the move. Declining to release here strands the session on a
	// gNB the UE has already left, with its PFCP session and IP lease held.
	t.Run("rolled back: nothing else would reap it", func(t *testing.T) {
		s, upf, sc := prepare(t)

		upf.err = errors.New("PFCP modify refused")

		if err := bind(t, s, sc.Ref); err == nil {
			t.Fatal("ModifyEPSSession succeeded though the UPF refused the modify")
		}

		upf.err = nil

		if err := s.ReleaseEPSSession(context.Background(), sc.Ref); err != nil {
			t.Fatalf("ReleaseEPSSession: %v", err)
		}

		if s.GetSession(sc.Ref) != nil {
			t.Error("the session survived a rolled-back commit, leaking its PFCP session and IP lease")
		}
	})
}

// Buffering the downlink of a session 5GS is serving over N3 blackholes it
// (TS 23.502 §4.11.2.2: the PDU session is released only once the move commits).
func TestDeactivateEPSSessionLeavesAnUncommittedMoveAlone(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establish5GS(t, s)

	if _, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID)); err != nil {
		t.Fatalf("prepare the move to EPS: %v", err)
	}

	modifies := len(upf.modifyCalls)

	if err := s.DeactivateEPSSession(ctx, sc.Ref); err != nil {
		t.Fatalf("DeactivateEPSSession: %v", err)
	}

	if len(upf.modifyCalls) != modifies {
		t.Error("the downlink of a session still served over N3 was put into buffering")
	}
}

// A refused PFCP modify leaves the data plane holding the source access's rules,
// so the model must go back to describing them too.
func TestRefusedCommitRestoresTheSourceBinding(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)

	sc.Mutex.Lock()
	before := ohcSummary(sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation)
	sc.Mutex.Unlock()

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	upf.err = errors.New("PFCP modify refused")

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err == nil {
		t.Fatal("the bind succeeded though the UPF refused the modify")
	}

	sc.Mutex.Lock()
	after := ohcSummary(sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation)
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Errorf("session on %s after a refused commit, want EPS", access)
	}

	if after != before {
		t.Errorf("downlink outer header = %s, want the eNB's %s: the data plane still holds it", after, before)
	}
}

func ohcSummary(o *models.OuterHeaderCreation) string {
	if o == nil {
		return "<none>"
	}

	return fmt.Sprintf("desc=%d teid=%#x v4=%s v6=%s s1u=%v", o.Description, o.TEID, o.IPv4Address, o.IPv6Address, o.S1U)
}

// A setup failure abandons the move, so the setup response that follows it names
// no commit. Binding a gNB downlink onto a session still recorded as EPS would
// leave the MME routing one the gNB now holds.
func TestBindingTo5GSIsRefusedWithoutACommit(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	fail, err := buildPDUSessionResourceSetupUnsuccessfulTransfer()
	if err != nil {
		t.Fatalf("build N2 setup failure: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupFail(ctx, ref, fail); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupFail: %v", err)
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err == nil {
		t.Fatal("a gNB downlink was bound onto a session the anchor still holds on EPS")
	}

	sc := s.GetSession(ref)

	sc.Mutex.Lock()
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Errorf("session on %s, want EPS", access)
	}
}
