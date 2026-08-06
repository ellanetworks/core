// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestModifyEPSSessionRejectsSupersededRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	superseded, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	live, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession (re-establish): %v", err)
	}

	if superseded.Ref == live.Ref {
		t.Fatalf("two sessions for EBI %d share ref %q", epsTestEBI, live.Ref)
	}

	upf.mu.Lock()
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}

	if err := s.ModifyEPSSession(ctx, superseded.Ref, enb); err == nil {
		t.Error("ModifyEPSSession(superseded ref) = nil, want error")
	}

	sc := s.GetSession(live.Ref)
	if sc == nil {
		t.Fatal("live EPS session is not in the pool")
	}

	sc.Mutex.Lock()
	an := sc.Tunnel.ANInformation
	sc.Mutex.Unlock()

	if an.TEID == enb.TEID && net.IP(enb.Addr.AsSlice()).Equal(an.IPv4Address) {
		t.Errorf("live session's AN endpoint = eNB S1-U %v/0x%x, want it unchanged", an.IPv4Address, an.TEID)
	}

	upf.mu.Lock()
	modifiesAfter := len(upf.modifyCalls)
	upf.mu.Unlock()

	if modifiesAfter != modifiesBefore {
		t.Errorf("UPF ModifySession calls = %d, want %d", modifiesAfter, modifiesBefore)
	}
}

func TestCreateEPSSessionKeeps5GSessionWithSameID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref5g, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), 5, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("CreateSmContext rejected the establishment: %d-byte N1 reject", len(rejectN1))
	}

	bearer, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if s.GetSession(ref5g) == nil {
		t.Error("5G session with PDU session id 5 released by an EPS create for EBI 5")
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Error("EPS session for EBI 5 is not live")
	}

	if bearer.Ref == ref5g {
		t.Errorf("EPS create returned the 5G session's ref %q", ref5g)
	}
}

func TestCreateSmContextKeepsEPSSessionWithSameID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	ref5g, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), 5, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("CreateSmContext rejected the establishment: %d-byte N1 reject", len(rejectN1))
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Error("EPS session for EBI 5 released by a 5G establishment for PDU session id 5")
	}

	if s.GetSession(ref5g) == nil {
		t.Error("5G session with PDU session id 5 is not live")
	}
}

// Leases must be keyed in the converged id space too, so coexisting sessions
// with the same wire id cannot share an address (TS 29.571 core-allocated
// range keeps them distinct).
func TestLeaseKeysDistinctAcrossAccesses(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), 5, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := s.CreateEPSSession(ctx, epsRequest(1)); err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	ids := store.allocSessionIDs()
	if len(ids) != 2 {
		t.Fatalf("lease allocations = %d, want 2", len(ids))
	}

	if ids[0] == ids[1] {
		t.Errorf("both accesses allocated under lease session id %d, want distinct ids", ids[0])
	}
}

// TS 24.501 §6.4.1.7 c): an initial request naming an identity already in use
// locally releases the existing session and proceeds. TS 23.501 §5.17.2.1 makes
// the identity span both systems, so the released one can be a PDN connection.
func TestInitialRequestLocallyReleasesThePDNConnectionHoldingTheIdentity(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeInitialRequest))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext(initial request): %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("initial request rejected with 5GSM cause %s, want the existing session released", rejectCause(t, rejectN1))
	}

	if ref == bearer.Ref {
		t.Error("the new session reused the released session's ref")
	}

	if s.GetSession(bearer.Ref) != nil {
		t.Error("the PDN connection's session survived the local release")
	}

	if released := mmeCb.releasedAway(); len(released) != 1 || released[0] != epsTestEBI {
		t.Errorf("MME told of released connections = %v, want [%d]", released, epsTestEBI)
	}
}

// A reference outlives a move, so the per-access entry points refuse a session
// served by the other access.
func TestEPSEntryPointsRefuseASessionMovedTo5GS(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeInitialRequest))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext(existing PDU session): %v", err)
	}

	if err := s.ModifyEPSSession(ctx, bearer.Ref, enbFTEID()); err == nil {
		t.Error("ModifyEPSSession on a session moved to 5GS = nil error, want a refusal")
	}

	if err := s.UpdateEPSSessionAMBR(ctx, bearer.Ref, models.MustParseBitRate("1 Mbps"), models.MustParseBitRate("1 Mbps")); err == nil {
		t.Error("UpdateEPSSessionAMBR on a session moved to 5GS = nil error, want a refusal")
	}

	if err := s.ReleaseEPSSession(ctx, bearer.Ref); err == nil {
		t.Error("ReleaseEPSSession on a session moved to 5GS = nil error, want a refusal")
	}

	if s.ServesEPS(ctx, bearer.Ref) {
		t.Error("ServesEPS = true for a session moved to 5GS, want false")
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Error("an EPS entry point tore down the session after the move")
	}
}

// TS 23.501 §5.17.2: a session that moves out and back keeps its address and its
// single lease, and each access is told once.
func TestRoundTripKeepsTheAddressAndLease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	sc := s.GetSession(ref)

	sc.Mutex.Lock()
	ip := sc.PDUIPV4Address.String()
	sc.Mutex.Unlock()

	// 5GS -> EPS, then bind the eNB so the source release drains.
	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	// EPS -> 5GS.
	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext(existing PDU session): %v", err)
	}

	sc.Mutex.Lock()
	back := sc.PDUIPV4Address.String()
	onEPS := sc.IsEPS()
	sc.Mutex.Unlock()

	if back != ip {
		t.Errorf("UE address after the round trip = %s, want the original %s", back, ip)
	}

	if onEPS {
		t.Error("session is on EPS after moving back to 5GS")
	}

	if ids := store.allocSessionIDs(); len(ids) != 1 {
		t.Errorf("lease allocations = %v, want the one from the first establishment", ids)
	}

	if moved := amfCb.movedAway(); len(moved) != 1 {
		t.Errorf("AMF told of moved sessions = %v, want exactly one", moved)
	}
}

// TS 24.501 §6.4.1.7 c) locally releases the session holding the identity. The
// access serving it has to be told, whichever one it is.
func TestSupersedingA5GSessionTellsTheAMF(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	first, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	second, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext(supersede): %v", err)
	}

	if first == second {
		t.Fatal("the superseding session reused the superseded ref")
	}

	if released := amfCb.releasedPDUSessions(); len(released) != 1 || released[0] != transferTestPDUSessionID {
		t.Errorf("AMF told of released sessions = %v, want [%d]", released, transferTestPDUSessionID)
	}
}
