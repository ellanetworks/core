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
