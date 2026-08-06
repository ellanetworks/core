// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

// enbFTEID is the S1-U endpoint an eNB reports once the default bearer is up.
func enbFTEID() models.FTEID {
	return models.FTEID{TEID: 0x7001, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 4})}
}

// transferTestPDUSessionID is the identity buildPDUSessionEstRequest names.
const transferTestPDUSessionID uint8 = 1

func epsTransferRequest(t *testing.T, requestType eps.RequestType) models.EPSBearerRequest {
	t.Helper()

	req := epsRequest(1)
	req.PDUSessionID = transferTestPDUSessionID
	req.RequestType = requestType
	req.Snssai = &models.Snssai{Sst: 1, Sd: "010203"}

	return req
}

func rejectCause(t *testing.T, raw []byte) fgs.GSMCause {
	t.Helper()

	msg, err := fgs.ParseMessage(raw)
	if err != nil {
		t.Fatalf("parse reject: %v", err)
	}

	reject, ok := msg.(*fgs.PDUSessionEstablishmentReject)
	if !ok {
		t.Fatalf("reject is %T, want *fgs.PDUSessionEstablishmentReject", msg)
	}

	return reject.Cause
}

// TS 23.502 §4.11.2.2 step 13.
func TestTransfer5GSToEPSKeepsSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil || rejectN1 != nil {
		t.Fatalf("CreateSmContext: %v (reject %d bytes)", err, len(rejectN1))
	}

	sc := s.GetSession(ref)
	if sc == nil {
		t.Fatal("5G session is not in the pool")
	}

	sc.Mutex.Lock()
	ip := sc.PDUIPV4Address.String()
	seid := sc.PFCPContext.LocalSEID
	ulTEID := sc.Tunnel.DataPath.UpLinkTunnel.TEID
	sc.Mutex.Unlock()

	upf.mu.Lock()
	establishesBefore := upf.lastEstablish
	upf.mu.Unlock()

	upf.mu.Lock()
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover))
	if err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	// The QoS change and the downlink suspend travel in one PFCP modification, so
	// a rejected transfer cannot leave the UPF holding half of it.
	upf.mu.Lock()
	modifies := len(upf.modifyCalls) - modifiesBefore
	upf.mu.Unlock()

	if modifies != 1 {
		t.Errorf("UPF modifications during the transfer = %d, want 1", modifies)
	}

	if bearer.Ref != ref {
		t.Errorf("bearer ref = %q, want the 5G session's %q", bearer.Ref, ref)
	}

	if bearer.IPv4.String() != ip {
		t.Errorf("UE address = %s, want the one it held on 5GS, %s", bearer.IPv4, ip)
	}

	if bearer.SGW.TEID != ulTEID {
		t.Errorf("S-GW S1-U TEID = %#x, want the anchor's uplink TEID %#x", bearer.SGW.TEID, ulTEID)
	}

	func() {
		sc.Mutex.Lock()
		defer sc.Mutex.Unlock()

		if !sc.IsEPS() {
			t.Error("session is still on 5GS after the transfer")
		}

		if sc.EBI != epsTestEBI || sc.PDUSessionID != transferTestPDUSessionID {
			t.Errorf("identity = ebi %d pdu-session-id %d, want %d and %d", sc.EBI, sc.PDUSessionID, epsTestEBI, transferTestPDUSessionID)
		}

		if sc.PFCPContext.LocalSEID != seid {
			t.Errorf("SEID = %d, want it preserved at %d", sc.PFCPContext.LocalSEID, seid)
		}

		if sc.Tunnel.DataPath.UpLinkTunnel.TEID != ulTEID {
			t.Errorf("uplink TEID = %#x, want it preserved at %#x", sc.Tunnel.DataPath.UpLinkTunnel.TEID, ulTEID)
		}

		if sc.PolicyData == nil || sc.PolicyData.Ambr.Uplink.Bps() == 0 {
			t.Error("transferred session has no policy from the target access")
		}
	}()

	if ids := store.allocSessionIDs(); len(ids) != 1 {
		t.Errorf("lease allocations = %v, want the one from the 5GS establishment", ids)
	}

	// TS 23.502 §4.11.2.2: step 14 follows step 13, so 5GS still routes the
	// session until the eNB downlink is bound.
	if moved := amfCb.movedAway(); len(moved) != 0 {
		t.Errorf("AMF told of moved sessions before the eNB bind = %v, want none", moved)
	}

	if err := s.ModifyEPSSession(ctx, sc.Ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	if moved := amfCb.movedAway(); len(moved) != 1 || moved[0] != transferTestPDUSessionID {
		t.Errorf("AMF told of moved sessions = %v, want [%d]", moved, transferTestPDUSessionID)
	}

	// TS 23.502 §4.11.2.2 step 14.
	amfCb.mu.Lock()
	releases := amfCb.transferReleases
	amfCb.mu.Unlock()

	if releases != 1 {
		t.Errorf("N2 releases for the moved session = %d, want 1", releases)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastEstablish != establishesBefore {
		t.Error("the transfer established a second UPF session")
	}
}

// TS 23.502 §4.11.2.3 step 9.
func TestTransferEPSTo5GSKeepsSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeInitialRequest))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	sc.Mutex.Lock()
	ip := sc.PDUIPV4Address.String()
	seid := sc.PFCPContext.LocalSEID
	ulTEID := sc.Tunnel.DataPath.UpLinkTunnel.TEID
	sc.Mutex.Unlock()

	upf.mu.Lock()
	establishesBefore := upf.lastEstablish
	upf.mu.Unlock()

	upf.mu.Lock()
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext(existing PDU session): %v", err)
	}

	// One PFCP modification for the whole move; see the 5GS→EPS direction.
	upf.mu.Lock()
	modifies := len(upf.modifyCalls) - modifiesBefore
	upf.mu.Unlock()

	if modifies != 1 {
		t.Errorf("UPF modifications during the transfer = %d, want 1", modifies)
	}

	if rejectN1 != nil {
		t.Fatalf("transfer rejected with 5GSM cause %s", rejectCause(t, rejectN1))
	}

	if ref != bearer.Ref {
		t.Errorf("SM context ref = %q, want the EPS session's %q", ref, bearer.Ref)
	}

	func() {
		sc.Mutex.Lock()
		defer sc.Mutex.Unlock()

		if sc.IsEPS() {
			t.Error("session is still on EPS after the transfer")
		}

		if sc.EBI != 0 {
			t.Errorf("EBI = %d, want it given up with the PDN connection", sc.EBI)
		}

		if sc.PDUSessionID != transferTestPDUSessionID {
			t.Errorf("PDU session id = %d, want %d", sc.PDUSessionID, transferTestPDUSessionID)
		}

		if sc.PDUIPV4Address.String() != ip {
			t.Errorf("UE address = %s, want the one it held on EPS, %s", sc.PDUIPV4Address, ip)
		}

		if sc.PFCPContext.LocalSEID != seid || sc.Tunnel.DataPath.UpLinkTunnel.TEID != ulTEID {
			t.Errorf("SEID/uplink TEID = %d/%#x, want them preserved at %d/%#x",
				sc.PFCPContext.LocalSEID, sc.Tunnel.DataPath.UpLinkTunnel.TEID, seid, ulTEID)
		}

		if sc.PolicyData == nil || sc.PolicyData.QosData.QFI == 0 {
			t.Error("transferred session has no 5GS QoS flow identifier")
		}
	}()

	if ids := store.allocSessionIDs(); len(ids) != 1 {
		t.Errorf("lease allocations = %v, want the one from the EPS establishment", ids)
	}

	// TS 23.502 §4.11.2.3: step 10 follows the user plane switch of step 9.
	if moved := mmeCb.movedAway(); len(moved) != 0 {
		t.Errorf("MME told of moved connections before the gNB bind = %v, want none", moved)
	}

	n2Data, err := buildPDUSessionResourceSetupResponseTransferIPv6(0x2222, net.ParseIP("10.0.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, sc.Ref, n2Data); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastEstablish != establishesBefore {
		t.Error("the transfer established a second UPF session")
	}
}

// ESM #54 (TS 24.301 §6.5.1.4 b) and 5GSM #54 (TS 24.501 §6.4.1.4 d).
func TestTransferOfUnknownSessionIsRefused(t *testing.T) {
	t.Run("to EPS", func(t *testing.T) {
		pcf, store, upf, amfCb := defaultFakes()
		s := newTestSMF(pcf, store, upf, amfCb)

		bearer, err := s.CreateEPSSession(context.Background(), epsTransferRequest(t, eps.RequestTypeHandover))
		if err == nil {
			t.Fatal("CreateEPSSession(handover) with no session to transfer = nil error")
		}

		if bearer.ESMCause != eps.ESMCausePDNConnectionDoesNotExist {
			t.Errorf("ESM cause = %s, want #54 PDN connection does not exist", bearer.ESMCause)
		}

		if ids := store.allocSessionIDs(); len(ids) != 0 {
			t.Errorf("lease allocations = %v, want none for a refused transfer", ids)
		}
	})

	t.Run("to 5GS", func(t *testing.T) {
		pcf, store, upf, amfCb := defaultFakes()
		s := newTestSMF(pcf, store, upf, amfCb)

		ref, rejectN1, err := s.CreateSmContext(context.Background(), testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
		if err == nil {
			t.Fatal("CreateSmContext(existing PDU session) with no session to transfer = nil error")
		}

		if ref != "" {
			t.Errorf("SM context ref = %q, want none", ref)
		}

		if cause := rejectCause(t, rejectN1); cause != fgs.GSMCausePDUSessionDoesNotExist {
			t.Errorf("5GSM cause = %s, want #54 PDU session does not exist", cause)
		}

		if ids := store.allocSessionIDs(); len(ids) != 0 {
			t.Errorf("lease allocations = %v, want none for a refused transfer", ids)
		}
	})
}

func TestTransferRefusedOnDataNetworkMismatch(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	req := epsTransferRequest(t, eps.RequestTypeHandover)
	req.APN = "ims"

	bearer, err := s.CreateEPSSession(ctx, req)
	if err == nil {
		t.Fatal("CreateEPSSession(handover) for another data network = nil error")
	}

	// The session exists, so #54 would invite the UE to discard state for a live
	// PDU session (TS 24.501 annex B).
	if bearer.ESMCause != eps.ESMCauseMissingOrUnknownAPN {
		t.Errorf("ESM cause = %s, want #27 missing or unknown APN", bearer.ESMCause)
	}

	if !errors.Is(err, smf.ErrSessionOnOtherDNN) {
		t.Errorf("error = %v, want it to wrap ErrSessionOnOtherDNN", err)
	}
}

func TestConcurrentTransfersDoNotBothCommit(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		ref       string
	)

	for range 2 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err == nil {
				mu.Lock()
				succeeded++
				ref = bearer.Ref
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if succeeded != 1 {
		t.Errorf("%d of 2 racing transfers committed, want exactly 1", succeeded)
	}

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	if moved := amfCb.movedAway(); len(moved) != 1 {
		t.Errorf("AMF told of moved sessions = %v, want exactly one", moved)
	}

	if fourG, _ := s.SessionCountByRAT(); fourG != 1 {
		t.Errorf("EPS session count = %d, want the one transferred session", fourG)
	}
}

// A transfer the UPF rejects leaves nothing half-applied: the session stays on
// the source access with its QoS, its downlink and its identity intact.
func TestTransferRolledBackWhenTheUPFRejectsIt(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	sc := s.GetSession(ref)

	sc.Mutex.Lock()
	qer := sc.Tunnel.DataPath.DownLinkTunnel.PDR.QER
	qfiBefore := qer.QFI
	forwBefore := sc.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction.Forw
	sc.Mutex.Unlock()

	upf.mu.Lock()
	upf.err = errors.New("UPF rejected the modification")
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err == nil {
		t.Fatal("CreateEPSSession(handover) with a failing UPF = nil error")
	}

	// The QoS change and the downlink suspend are one modification, so a rejected
	// transfer cannot leave the UPF holding half of it.
	upf.mu.Lock()
	modifies := len(upf.modifyCalls) - modifiesBefore
	upf.mu.Unlock()

	if modifies != 1 {
		t.Errorf("UPF modifications during the transfer = %d, want 1", modifies)
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.IsEPS() {
		t.Error("the session moved to EPS despite the UPF rejecting the change")
	}

	if qer.QFI != qfiBefore {
		t.Errorf("QER QFI = %d, want it restored to %d", qer.QFI, qfiBefore)
	}

	if got := sc.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction.Forw; got != forwBefore {
		t.Errorf("downlink Forw = %v, want it restored to %v", got, forwBefore)
	}

	if sc.EBI != 0 {
		t.Errorf("EBI = %d, want the target identity given up", sc.EBI)
	}

	if moved := amfCb.movedAway(); len(moved) != 0 {
		t.Errorf("AMF told of moved sessions = %v, want none", moved)
	}
}

// A target access that never binds must not leave the session on one access
// while the other still routes it, so the bind is supervised and its expiry
// releases the session.
func TestTransferReleasedWhenTheTargetNeverBinds(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithTransferBindTimeout(5*time.Millisecond))
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	// No ModifyEPSSession: the eNB never answers.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.GetSession(ref) == nil {
			break
		}

		time.Sleep(5 * time.Millisecond)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("the session survived a target access that never bound")
	}

	// The release drains the pending, so 5GS is told to stop routing it.
	if moved := amfCb.movedAway(); len(moved) != 1 || moved[0] != transferTestPDUSessionID {
		t.Errorf("AMF told of moved sessions = %v, want [%d]", moved, transferTestPDUSessionID)
	}
}

// buildPDUSessionResourceSetupUnsuccessfulTransferForTest encodes the N2
// container a gNB returns when it cannot set up a PDU session resource.
func buildPDUSessionResourceSetupUnsuccessfulTransferForTest() ([]byte, error) {
	t := libngap.PDUSessionResourceSetupUnsuccessfulTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: 0},
	}

	b, err := t.Marshal()

	return b, err
}

// A stale 5G setup failure arriving after the session moved to EPS says nothing
// about its EPS target, so it must not release it. Two guards produce this: the
// failure path requires the session to still be on 5GS, and the 5GS release
// entry point refuses a session on EPS.
func TestStale5GSetupFailureDoesNotReleaseAnEPSSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	n2Fail, err := buildPDUSessionResourceSetupUnsuccessfulTransferForTest()
	if err != nil {
		t.Fatalf("build unsuccessful transfer: %v", err)
	}

	_ = s.UpdateSmContextN2InfoPduResSetupFail(ctx, ref, n2Fail)

	if s.GetSession(ref) == nil {
		t.Fatal("a stale 5G setup failure released the session after it moved to EPS")
	}
}

// C4: a teardown reaching the session before its target bound still has to drop
// the routing the source access holds.
func TestTeardownDrainsThePendingSourceRelease(t *testing.T) {
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

	if moved := mmeCb.movedAway(); len(moved) != 0 {
		t.Fatalf("MME told of moved connections before the gNB bind = %v, want none", moved)
	}

	// The UE releases before the gNB ever answers.
	if err := s.ReleaseSmContext(ctx, bearer.Ref); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}
}

// movedFromEPSTo5GS establishes a PDN connection and moves it to 5GS, leaving the
// gNB unbound so the source access still routes the session. It returns the MME
// callback the SMF was given and the session's ref, which the move preserves.
func movedFromEPSTo5GS(t *testing.T, s *smf.SMF) (*fakeMME, string) {
	t.Helper()

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

	if moved := mmeCb.movedAway(); len(moved) != 0 {
		t.Fatalf("MME told of moved connections before the gNB bind = %v, want none", moved)
	}

	return mmeCb, bearer.Ref
}

// A UE-signalled release completing before the target access bound still has to
// drop the routing the source access holds.
func TestNASReleaseCompleteDrainsThePendingSourceRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb, ref := movedFromEPSTo5GS(t, s)

	const pti = 5

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(transferTestPDUSessionID, pti)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg(PDU session release request): %v", err)
	}

	// The session is still in the pool, mid release procedure, so the source
	// access keeps routing it.
	if moved := mmeCb.movedAway(); len(moved) != 0 {
		t.Fatalf("MME told of moved connections before the release completed = %v, want none", moved)
	}

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseComplete(transferTestPDUSessionID, pti)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg(PDU session release complete): %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("the session survived the release complete")
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}
}

// An N2 release response tearing the session down before the target access bound
// still has to drop the routing the source access holds.
func TestN2ReleaseResponseDrainsThePendingSourceRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb, ref := movedFromEPSTo5GS(t, s)

	if err := s.UpdateSmContextN2InfoPduResRelRsp(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResRelRsp: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("the session survived the N2 release response")
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}
}

// TS 24.501 §6.4.1.7 c): an initial request supersedes the session holding the
// identity. One superseded before its target access bound still holds the source
// access's routing.
func TestSupersedingATransferredSessionDrainsThePendingSourceRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb, ref := movedFromEPSTo5GS(t, s)

	newRef, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext(initial request): %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("initial request rejected with 5GSM cause %s, want the existing session superseded", rejectCause(t, rejectN1))
	}

	if newRef == ref {
		t.Errorf("the superseding session reused the superseded session's ref %q", ref)
	}

	if s.GetSession(ref) != nil {
		t.Error("the superseded session is still in the pool")
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}
}

// TS 23.501 §5.17.2: a session moved out and back before its first target bound
// is returned to the access it is recorded as having left, so only the access it
// left last is told to stop routing it.
func TestRoundTripWithoutABindTellsOnlyTheAccessLeftLast(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	// No ModifyEPSSession: the eNB never binds before the UE moves back.
	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext(existing PDU session): %v", err)
	}

	n2Data, err := buildPDUSessionResourceSetupResponseTransferIPv6(0x3333, net.ParseIP("10.0.0.11"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2Data); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}

	// 5GS serves the session again under the same ref, so the entry recorded when
	// it first left 5GS names the live session.
	if moved := amfCb.movedAway(); len(moved) != 0 {
		t.Errorf("AMF told of moved sessions = %v, want none", moved)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("the session was released by the round trip")
	}
}

// The supervision that releases a session whose target access never binds is
// stopped by the bind, so a bound session outlives the timeout.
func TestTargetBindStopsTheTransferSupervision(t *testing.T) {
	const bindTimeout = 10 * time.Millisecond

	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithTransferBindTimeout(bindTimeout))
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	time.Sleep(20 * bindTimeout)

	if s.GetSession(ref) == nil {
		t.Fatalf("the session was released %s after the eNB bound, want it kept", 20*bindTimeout)
	}

	if fourG, _ := s.SessionCountByRAT(); fourG != 1 {
		t.Errorf("EPS session count = %d, want the one bound session", fourG)
	}
}

// gatedUPF holds one session modification open, so a caller can act while the
// transfer that issued it is in flight.
type gatedUPF struct {
	*fakeUPF

	arm     chan struct{}
	entered chan struct{}
	proceed chan struct{}
}

func newGatedUPF(upf *fakeUPF) *gatedUPF {
	g := &gatedUPF{
		fakeUPF: upf,
		arm:     make(chan struct{}, 1),
		entered: make(chan struct{}),
		proceed: make(chan struct{}),
	}
	g.arm <- struct{}{}

	return g
}

func (g *gatedUPF) ModifySession(ctx context.Context, req *models.ModifyRequest) error {
	select {
	case <-g.arm:
		close(g.entered)
		<-g.proceed
	default:
	}

	return g.fakeUPF.ModifySession(ctx, req)
}

// A release and a transfer both rewrite the session across blocking UPF calls, so
// one has to lose. The session is left on exactly one access, whichever wins.
func TestReleaseRacingATransferLeavesOneOutcome(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	gated := newGatedUPF(upf)
	s := newTestSMF(pcf, store, gated, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	transferred := make(chan error, 1)

	go func() {
		_, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover))
		transferred <- err
	}()

	<-gated.entered

	released := make(chan error, 1)

	go func() { released <- s.ReleaseSmContext(ctx, ref) }()

	close(gated.proceed)

	transferErr := <-transferred
	releaseErr := <-released

	if transferErr != nil && releaseErr != nil {
		t.Fatalf("both procedures failed: transfer %v, release %v; want exactly one to commit", transferErr, releaseErr)
	}

	sc := s.GetSession(ref)

	switch {
	case transferErr == nil && releaseErr == nil:
		t.Error("the transfer and the release both committed against the same session")
	case transferErr == nil:
		if sc == nil {
			t.Fatal("the session is gone after a committed transfer and a refused release")
		}

		sc.Mutex.Lock()
		onEPS := sc.IsEPS()
		sc.Mutex.Unlock()

		if !onEPS {
			t.Error("session is on 5GS after a committed transfer to EPS")
		}
	default:
		if sc != nil {
			t.Error("the session is still in the pool after a committed release")
		}
	}
}

// TS 24.501 §6.3.3.5: the release command is retransmitted four times and then
// abandoned, tearing the session down. One abandoned before its target access
// bound still holds the source access's routing.
func TestReleaseRetransmissionAbortDrainsThePendingSourceRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithT3592(2*time.Millisecond))
	ctx := context.Background()

	mmeCb, ref := movedFromEPSTo5GS(t, s)

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(transferTestPDUSessionID, 5)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg(PDU session release request): %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && s.GetSession(ref) != nil {
		time.Sleep(2 * time.Millisecond)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("the session survived the release command retransmission limit")
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}
}

// A transfer's commit spans several blocking UPF calls. An initial request
// naming the same identity meanwhile leaves the transferred session alone, so the
// supersede cannot tear down state the move is part-way through writing; the
// identity stays in use and the establishment is refused.
func TestSupersedeSkippedWhileATransferHoldsTheSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	gated := newGatedUPF(upf)
	s := newTestSMF(pcf, store, gated, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	transferred := make(chan error, 1)

	go func() {
		_, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover))
		transferred <- err
	}()

	<-gated.entered

	newRef, _, createErr := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())

	close(gated.proceed)

	if transferErr := <-transferred; transferErr != nil {
		t.Fatalf("CreateEPSSession(handover): %v", transferErr)
	}

	if createErr == nil {
		t.Errorf("CreateSmContext(initial request) during a transfer = ref %q and nil error, want a refusal", newRef)
	}

	sc := s.GetSession(ref)
	if sc == nil {
		t.Fatal("the transferred session was superseded while the transfer held it")
	}

	sc.Mutex.Lock()
	onEPS := sc.IsEPS()
	sc.Mutex.Unlock()

	if !onEPS {
		t.Error("the transferred session is on 5GS, want it on EPS")
	}
}
