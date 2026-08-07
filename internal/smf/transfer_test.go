// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// interworkingFakes is one subscriber and one data network the same on both
// accesses, which is what makes a session movable between them.
func interworkingFakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF, *fakeMME) {
	pcf, store, upf, amfCb := defaultFakes()
	store.allocatedIP = netip.AddrFrom4([4]byte{10, 45, 0, 7})
	store.releasedIP = store.allocatedIP

	return pcf, store, upf, amfCb, &fakeMME{}
}

// epsMove is a PDN CONNECTIVITY REQUEST with request type "handover": the UE
// naming a PDU session it holds in 5GS and asking the anchor to move it.
func epsMove(pduSessionID uint8) models.EPSBearerRequest {
	req := epsRequest(1)
	req.APN = testDNN
	req.PDUSessionID = pduSessionID
	req.RequestType = eps.RequestTypeHandover

	return req
}

// movedPDUSessionID is the identity the UE allocated for the session these tests
// move between accesses; it is what correlates the two accesses
// (TS 23.501 §5.17.2.1).
const movedPDUSessionID uint8 = 3

// establish5GS brings up a 5G session with its downlink bound to a gNB, which is
// the state a UE is in before it moves the session to EPS.
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

// sessionInvariants is what a move must preserve: the UE keeps its address
// because the anchor keeps the session, its UPF state and its uplink tunnel
// (TS 23.502 §4.11.2.2 step 14).
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

// A UE that re-attaches in EPS with request type "handover" keeps its address,
// because the anchor moves the session it already has rather than establishing a
// new one. Nothing new is allocated: no second UPF session, no second lease, and
// the uplink F-TEID the RAN sends to is unchanged.
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

	// The downlink stays on 5GS until the eNB endpoint arrives: TS 23.401 §5.10.2
	// step 5 holds it at the PDN GW until step 13a.
	sc.Mutex.Lock()
	access := sc.Access
	sc.Mutex.Unlock()

	if access != smf.Access5G {
		t.Error("the session left 5GS before the eNB bound its downlink")
	}

	// The eNB's S1-U endpoint commits the move.
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

	// One session, one lease: the move allocated neither.
	if s.SessionCount() != 1 {
		t.Errorf("sessions = %d, want 1: the move must not create a second", s.SessionCount())
	}

	if got := len(store.ops()); got != establishes {
		t.Errorf("IP pool operations = %d, want %d: the move must not touch the lease", got, establishes)
	}

	// The access the session left is told to stop routing it, and to release the
	// radio resources it still holds there — without releasing the session.
	calls := amfCb.transferred()
	if len(calls) != 1 {
		t.Fatalf("AMF SessionTransferred calls = %d, want 1", len(calls))
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

// The mirror: a UE moving back to 5GS with request type "existing PDU session"
// keeps the same address and the same anchored session.
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

	// The gNB's N3 endpoint commits the move.
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

	// The EPS bearer the UE left no longer exists, so the identity is dropped and
	// the next attach may legitimately allocate it again.
	if ebi != 0 {
		t.Errorf("session still holds EPS bearer identity %d after moving to 5GS", ebi)
	}

	if s.SessionCount() != 1 {
		t.Errorf("sessions = %d, want 1", s.SessionCount())
	}

	if got := len(store.ops()); got != establishes {
		t.Errorf("IP pool operations = %d, want %d: the move must not touch the lease", got, establishes)
	}

	calls := mmeCb.transferred()
	if len(calls) != 1 {
		t.Fatalf("MME SessionTransferred calls = %d, want 1", len(calls))
	}

	if calls[0].ref != sc.Ref || calls[0].ebi != epsTestEBI {
		t.Errorf("MME told about ref %q ebi %d, want %q / %d", calls[0].ref, calls[0].ebi, sc.Ref, epsTestEBI)
	}

	if s.GetSession(sc.Ref) == nil {
		t.Error("the session was released by the move")
	}
}

// A move of something the anchor does not hold draws #54 on either access, which
// tells the UE to establish rather than retry (TS 24.301 §6.5.1.6 b,
// TS 24.501 §6.4.1.7 d). A session it does hold but cannot move as asked draws
// #26 instead: #54 would be untrue.
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

		// A UE that sent no PDU session identity in its PCO has nothing to correlate
		// the two accesses with, so there is no session to move.
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

		// The network does know this session, so #54 would be untrue.
		assertGSMCause(t, reject, fgs.GSMCauseInsufficientResources)
	})
}

// A UE moving to 5GS names the slice it holds for the session (TS 24.501
// §6.4.1.2 c)2). Naming another one names another session, so the move is
// refused rather than silently re-slicing the one the anchor holds.
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

	// The session is untouched and still movable.
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

// Two moves of one session cannot run at once: each re-points the same user
// plane, and each tells an access to forget the session, so the second would
// leave a session no control plane owns.
func TestOneTransferAtATime(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	establish5GS(t, s)

	if _, err := s.CreateEPSSession(ctx, epsMove(3)); err != nil {
		t.Fatalf("first move: %v", err)
	}

	// The eNB has not bound the downlink yet, so the first move is still in flight.
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
