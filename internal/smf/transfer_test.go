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

// bindGNB answers the N2 PDU Session Resource Setup Request, which is where a
// move onto 5GS commits (TS 23.502 §4.3.2.2.1 step 16a NOTE 11).
func bindGNB(t *testing.T, s *smf.SMF, ref string) {
	t.Helper()

	n2Data, err := buildPDUSessionResourceSetupResponseTransferIPv6(0x2222, net.ParseIP("10.0.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(context.Background(), ref, n2Data); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}
}

// onEPS reports the access the session is on.
func onEPS(t *testing.T, s *smf.SMF, ref string) bool {
	t.Helper()

	sc := s.GetSession(ref)
	if sc == nil {
		t.Fatalf("session %q is not in the pool", ref)
	}

	defer sc.LockForTest()()

	return sc.IsEPS()
}

// TS 23.502 §4.11.2.2 step 13, TS 23.401 §5.10.2: the request moves nothing —
// the PDN GW switches the downlink at step 13a, when the eNB binds.
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

	unlock := sc.LockForTest()
	ip := sc.PDUIPV4Address.String()
	seid := sc.UPFSession.SEID
	ulTEID := sc.Tunnel.DataPath.UpLinkTunnel.TEID
	qfi := sc.PolicyData.QosData.QFI

	unlock()

	appliesBefore := upf.applyCount()

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover))
	if err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	if applies := upf.applyCount() - appliesBefore; applies != 0 {
		t.Errorf("UPF session statements for the transfer request = %d, want 0: the downlink switches at the bind", applies)
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

	// TS 23.401 §5.10.2 step 5: the PDN GW does not yet send downlink packets to
	// the S-GW, so the session is still the one 5GS serves.
	if onEPS(t, s, ref) {
		t.Error("access after the transfer request = EPS, want 5GS until the eNB binds")
	}

	if !s.ServesEPS(ctx, ref) {
		t.Error("ServesEPS during a transfer onto EPS = false, want true: the MME holds the bearer")
	}

	if moved := amfCb.movedAway(); len(moved) != 0 {
		t.Errorf("AMF told of moved sessions before the eNB bind = %v, want none", moved)
	}

	if err := s.ModifyEPSSession(ctx, sc.Ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	if applies := upf.applyCount() - appliesBefore; applies != 1 {
		t.Errorf("UPF session statements for the request and the bind = %d, want 1", applies)
	}

	func() {
		defer sc.LockForTest()()

		if !sc.IsEPS() {
			t.Errorf("access after the bind = %s, want EPS", sc.Access)
		}

		if sc.EBI != epsTestEBI || sc.PDUSessionID != transferTestPDUSessionID {
			t.Errorf("identity = ebi %d pdu-session-id %d, want %d and %d", sc.EBI, sc.PDUSessionID, epsTestEBI, transferTestPDUSessionID)
		}

		if sc.UPFSession.SEID != seid {
			t.Errorf("SEID = %d, want it preserved at %d", sc.UPFSession.SEID, seid)
		}

		if sc.Tunnel.DataPath.UpLinkTunnel.TEID != ulTEID {
			t.Errorf("uplink TEID = %#x, want it preserved at %#x", sc.Tunnel.DataPath.UpLinkTunnel.TEID, ulTEID)
		}

		if sc.PolicyData == nil || sc.PolicyData.Ambr.Uplink.Bps() == 0 {
			t.Fatal("transferred session has no policy from the target access")
		}

		if !sc.PolicyData.Ambr.Uplink.Equal(models.MustParseBitRate("1 Gbps")) {
			t.Errorf("Session-AMBR uplink = %s, want the target access's 1 Gbps", sc.PolicyData.Ambr.Uplink)
		}

		if sc.PolicyData.QosData.QFI != qfi {
			t.Errorf("QFI = %d, want it stable across the move at %d", sc.PolicyData.QosData.QFI, qfi)
		}
	}()

	if ids := store.allocSessionIDs(); len(ids) != 1 {
		t.Errorf("lease allocations = %v, want the one from the 5GS establishment", ids)
	}

	// TS 23.502 §4.11.2.2 step 14 follows step 13.
	if moved := amfCb.movedAway(); len(moved) != 1 || moved[0] != transferTestPDUSessionID {
		t.Errorf("AMF told of moved sessions = %v, want [%d]", moved, transferTestPDUSessionID)
	}

	amfCb.mu.Lock()
	releases := amfCb.transferReleases
	amfCb.mu.Unlock()

	if releases != 1 {
		t.Errorf("N2 releases for the moved session = %d, want 1", releases)
	}

	// The move never creates a second UPF session: the one the establishment made
	// is the one the bind converges.
	for _, state := range upf.applies() {
		if state.SEID != seid {
			t.Errorf("UPF session statement names SEID %d, want the session kept at %d", state.SEID, seid)
		}
	}
}

// TS 23.502 §4.11.2.3 step 9 defers the user plane switch to §4.3.2.2.1
// step 16a, so the request moves nothing.
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

	unlock := sc.LockForTest()
	ip := sc.PDUIPV4Address.String()
	seid := sc.UPFSession.SEID
	ulTEID := sc.Tunnel.DataPath.UpLinkTunnel.TEID

	unlock()

	appliesBefore := upf.applyCount()

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext(existing PDU session): %v", err)
	}

	if applies := upf.applyCount() - appliesBefore; applies != 0 {
		t.Errorf("UPF session statements for the transfer request = %d, want 0: the downlink switches at the bind", applies)
	}

	if rejectN1 != nil {
		t.Fatalf("transfer rejected with 5GSM cause %s", rejectCause(t, rejectN1))
	}

	if ref != bearer.Ref {
		t.Errorf("SM context ref = %q, want the EPS session's %q", ref, bearer.Ref)
	}

	if !onEPS(t, s, ref) {
		t.Error("access after the transfer request = 5GS, want EPS until the gNB binds")
	}

	// EPS still routes the session, so its policy is the one in force until the
	// gNB binds.
	unlock = sc.LockForTest()
	windowAmbr := sc.PolicyData.Ambr.Uplink

	unlock()

	if !windowAmbr.Equal(models.MustParseBitRate("1 Gbps")) {
		t.Errorf("Session-AMBR uplink during the transfer window = %s, want the EPS access's 1 Gbps", windowAmbr)
	}

	// TS 23.502 §4.11.2.3 step 10 follows the user plane switch of step 9.
	if moved := mmeCb.movedAway(); len(moved) != 0 {
		t.Errorf("MME told of moved connections before the gNB bind = %v, want none", moved)
	}

	bindGNB(t, s, ref)

	if applies := upf.applyCount() - appliesBefore; applies != 1 {
		t.Errorf("UPF session statements for the request and the bind = %d, want 1", applies)
	}

	func() {
		defer sc.LockForTest()()

		if sc.IsEPS() {
			t.Errorf("access after the bind = %s, want 5GS", sc.Access)
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

		if sc.UPFSession.SEID != seid || sc.Tunnel.DataPath.UpLinkTunnel.TEID != ulTEID {
			t.Errorf("SEID/uplink TEID = %d/%#x, want them preserved at %d/%#x",
				sc.UPFSession.SEID, sc.Tunnel.DataPath.UpLinkTunnel.TEID, seid, ulTEID)
		}

		if sc.PolicyData == nil || sc.PolicyData.QosData.QFI == 0 {
			t.Fatal("transferred session has no 5GS QoS flow identifier")
		}

		if !sc.PolicyData.Ambr.Uplink.Equal(models.MustParseBitRate("100 Mbps")) {
			t.Errorf("Session-AMBR uplink after the bind = %s, want the target access's 100 Mbps", sc.PolicyData.Ambr.Uplink)
		}
	}()

	if ids := store.allocSessionIDs(); len(ids) != 1 {
		t.Errorf("lease allocations = %v, want the one from the EPS establishment", ids)
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}

	for _, state := range upf.applies() {
		if state.SEID != seid {
			t.Errorf("UPF session statement names SEID %d, want the session kept at %d", state.SEID, seid)
		}
	}
}

// ESM #54 (TS 24.301 §6.5.1.6 b) and 5GSM #54 (TS 24.501 §6.4.1.7 d).
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

	// TS 24.301 §6.5.1.6 b) mandates #54 for a request type "handover" naming a PDN
	// connection the network holds no information about, and §6.5.1.4.3 exempts #54
	// from the back-off #27 imposes for 12 minutes.
	if bearer.ESMCause != eps.ESMCausePDNConnectionDoesNotExist {
		t.Errorf("ESM cause = %s, want #54 PDN connection does not exist", bearer.ESMCause)
	}

	if !errors.Is(err, smf.ErrSessionOnOtherDNN) {
		t.Errorf("error = %v, want it to wrap ErrSessionOnOtherDNN", err)
	}
}

// A move the target access has not bound already answers for that access, so a
// second request for it must be refused: two accepted requests hand two accesses
// a bearer for one session.
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
		t.Errorf("%d of 2 racing transfers were accepted, want exactly 1", succeeded)
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

// A serialised second request is refused for the same reason as a racing one.
func TestASecondTransferRequestIsRefusedBeforeTheTargetBinds(t *testing.T) {
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

	req := epsTransferRequest(t, eps.RequestTypeHandover)
	req.EPSBearerIdentity = epsTestEBI + 1

	bearer, err := s.CreateEPSSession(ctx, req)
	if err == nil {
		t.Fatal("a second CreateEPSSession(handover) before the eNB bound = nil error, want a refusal")
	}

	if bearer.ESMCause != eps.ESMCausePDNConnectionDoesNotExist {
		t.Errorf("ESM cause = %s, want #54 PDN connection does not exist", bearer.ESMCause)
	}

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc := s.GetSession(ref)

	defer sc.LockForTest()()

	if sc.EBI != epsTestEBI {
		t.Errorf("EBI after the bind = %d, want the first request's %d", sc.EBI, epsTestEBI)
	}
}

// The identity is claimed at the bind, so a move the target abandons leaves it
// free for whoever asks next.
func TestTransferClaimsTheBearerIdentityOnlyAtTheBind(t *testing.T) {
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

	sc := s.GetSession(ref)

	unlock := sc.LockForTest()
	held := sc.EBI

	unlock()

	if held != 0 {
		t.Errorf("EBI during the transfer window = %d, want 0: the claim belongs to the bind", held)
	}

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	defer sc.LockForTest()()

	if sc.EBI != epsTestEBI {
		t.Errorf("EBI after the bind = %d, want %d", sc.EBI, epsTestEBI)
	}
}

// TS 23.401 §5.10.2 step 5: the PDN GW does not yet send downlink packets to the
// S-GW, and it does not stop sending them to the source access either, so the
// move opens no data gap.
func TestTransferLeavesTheSourceDownlinkForwarding(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	bindGNB(t, s, ref)

	sc := s.GetSession(ref)

	unlock := sc.LockForTest()
	gnbTEID := sc.Tunnel.ANInformation.TEID
	gnbIP := sc.Tunnel.ANInformation.IPv6Address.String()

	unlock()

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	defer sc.LockForTest()()

	far := sc.Tunnel.DataPath.DownLinkTunnel.PDR.FAR

	if !far.ApplyAction.Forw || far.ApplyAction.Buff {
		t.Errorf("downlink apply action during the transfer window = %+v, want it still forwarding", far.ApplyAction)
	}

	if sc.Tunnel.ANInformation.TEID != gnbTEID || sc.Tunnel.ANInformation.IPv6Address.String() != gnbIP {
		t.Errorf("AN endpoint during the transfer window = %v/%#x, want the gNB's %s/%#x",
			sc.Tunnel.ANInformation.IPv6Address, sc.Tunnel.ANInformation.TEID, gnbIP, gnbTEID)
	}
}

// A bind whose UPF modification is refused leaves nothing half-applied: the
// session stays on the access serving it, with its QoS and its identity intact.
func TestTransferRolledBackWhenTheBindIsRefused(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	sc := s.GetSession(ref)

	unlock := sc.LockForTest()
	qer := sc.Tunnel.DataPath.DownLinkTunnel.PDR.QER
	qfiBefore := qer.QFI
	mbrBefore := *qer.MBR

	unlock()

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	upf.mu.Lock()
	upf.err = errors.New("UPF rejected the modification")
	upf.mu.Unlock()

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err == nil {
		t.Fatal("ModifyEPSSession with a failing UPF = nil error")
	}

	defer sc.LockForTest()()

	if sc.IsEPS() {
		t.Error("the session moved to EPS despite the UPF rejecting the bind")
	}

	if qer.QFI != qfiBefore {
		t.Errorf("QER QFI = %d, want it restored to %d", qer.QFI, qfiBefore)
	}

	if *qer.MBR != mbrBefore {
		t.Errorf("QER MBR = %+v, want it restored to %+v", *qer.MBR, mbrBefore)
	}

	if sc.EBI != 0 {
		t.Errorf("EBI = %d, want the target identity given up", sc.EBI)
	}

	if moved := amfCb.movedAway(); len(moved) != 0 {
		t.Errorf("AMF told of moved sessions = %v, want none", moved)
	}
}

// A target access that never binds leaves the session where it is: the downlink
// was never switched, so there is nothing to unwind and nothing to release.
func TestTransferLeavesTheSessionOnItsSourceWhenTheTargetNeverBinds(t *testing.T) {
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

	// No ModifyEPSSession: the eNB never answers.
	time.Sleep(50 * time.Millisecond)

	if s.GetSession(ref) == nil {
		t.Fatal("the session was released after a target access that never bound")
	}

	if onEPS(t, s, ref) {
		t.Error("access = EPS after a target access that never bound, want 5GS")
	}

	if moved := amfCb.movedAway(); len(moved) != 0 {
		t.Errorf("AMF told of moved sessions = %v, want none: the move never committed", moved)
	}

	if released := amfCb.releasedPDUSessions(); len(released) != 0 {
		t.Errorf("AMF told of released sessions = %v, want none: the session survives on 5GS", released)
	}

	if released := mmeCb.releasedAway(); len(released) != 0 {
		t.Errorf("MME told of released connections = %v, want none", released)
	}
}

// TS 24.301 §5.5.1.2: a 4G attach the MME abandons releases the bearer it was
// handed. The 5GS session it names is not the abandoned leg, so it survives.
func TestReleaseEPSSessionDuringATransferOntoEPSKeepsThe5GSSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	bindGNB(t, s, ref)

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	deletesBefore := len(upf.deleteCalls)

	if err := s.ReleaseEPSSession(ctx, ref); err != nil {
		t.Errorf("ReleaseEPSSession during a transfer onto EPS = %v, want nil: the abandoned leg is dropped", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("the 5GS session was released by a release of the abandoned EPS leg")
	}

	if onEPS(t, s, ref) {
		t.Error("access after the abandoned EPS leg = EPS, want 5GS")
	}

	if s.ServesEPS(ctx, ref) {
		t.Error("ServesEPS after the abandoned EPS leg = true, want false")
	}

	upf.mu.Lock()
	deletes := len(upf.deleteCalls) - deletesBefore
	upf.mu.Unlock()

	if deletes != 0 {
		t.Errorf("UPF session deletions = %d, want 0", deletes)
	}

	// The leg is gone, so the UE can ask for the move again.
	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Errorf("CreateEPSSession(handover) after the abandoned leg = %v, want it accepted", err)
	}
}

// TS 24.501 §6.4.1.2: a 5GS registration the AMF abandons releases the PDU
// session it was handed. The PDN connection it names is not the abandoned leg.
func TestReleaseSmContextDuringATransferOnto5GSKeepsTheEPSSession(t *testing.T) {
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

	if err := s.ReleaseSmContext(ctx, bearer.Ref); err != nil {
		t.Errorf("ReleaseSmContext during a transfer onto 5GS = %v, want nil: the abandoned leg is dropped", err)
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Fatal("the PDN connection was released by a release of the abandoned 5GS leg")
	}

	if !onEPS(t, s, bearer.Ref) {
		t.Error("access after the abandoned 5GS leg = 5GS, want EPS")
	}

	if moved := mmeCb.movedAway(); len(moved) != 0 {
		t.Errorf("MME told of moved connections = %v, want none: the move never committed", moved)
	}

	if released := mmeCb.releasedAway(); len(released) != 0 {
		t.Errorf("MME told of released connections = %v, want none: the PDN connection survives", released)
	}
}

// A release arriving on the access serving the session releases it, whatever
// move is outstanding.
func TestReleaseOnTheServingAccessReleasesADSessionWithAnOutstandingTransfer(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	bindGNB(t, s, ref)

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	if err := s.ReleaseSmContext(ctx, ref); err != nil {
		t.Fatalf("ReleaseSmContext on the serving access: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Error("the session survived a release on the access serving it")
	}
}

// deactivateSmContext drops the tunnel and the PFCP context when the UPF refuses
// the modification, so a bind that assumed the request-time user plane would
// dereference neither.
func TestBindRefusedWhenTheUserPlaneWentAwayDuringTheTransferWindow(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	bindGNB(t, s, ref)

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	upf.mu.Lock()
	upf.err = errors.New("UPF has no such session")
	upf.mu.Unlock()

	if err := s.DeactivateSmContext(ctx, ref); err == nil {
		t.Fatal("DeactivateSmContext with a failing UPF = nil error")
	}

	upf.mu.Lock()
	upf.err = nil
	upf.mu.Unlock()

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err == nil {
		t.Error("ModifyEPSSession after the user plane went away = nil error, want a refusal")
	}

	sc := s.GetSession(ref)

	defer sc.LockForTest()()

	if sc.IsEPS() {
		t.Error("the session moved to EPS on a bind with no user plane to move")
	}

	if sc.EBI != 0 {
		t.Errorf("EBI = %d, want no identity claimed by a refused bind", sc.EBI)
	}
}

// TS 24.501 §6.4.1.7 c): an initial request supersedes the session holding the
// identity. One with a move outstanding is held by two accesses, and both
// installed state from the answer they were given.
func TestSupersedingASessionWithAnOutstandingTransferTellsBothAccesses(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	bindGNB(t, s, ref)

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

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

	if released := amfCb.releasedPDUSessions(); len(released) != 1 || released[0] != transferTestPDUSessionID {
		t.Errorf("AMF told of released sessions = %v, want [%d]", released, transferTestPDUSessionID)
	}

	if released := mmeCb.releasedAway(); len(released) != 1 || released[0] != epsTestEBI {
		t.Errorf("MME told of released connections = %v, want [%d]: it holds the bearer it was handed", released, epsTestEBI)
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
// about its EPS target, so it must not release it.
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

	if err := s.ModifyEPSSession(ctx, ref, enbFTEID()); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	n2Fail, err := buildPDUSessionResourceSetupUnsuccessfulTransferForTest()
	if err != nil {
		t.Fatalf("build unsuccessful transfer: %v", err)
	}

	_ = s.UpdateSmContextN2InfoPduResSetupFail(ctx, ref, n2Fail)

	if s.GetSession(ref) == nil {
		t.Fatal("a stale 5G setup failure released the session after it moved to EPS")
	}

	if !onEPS(t, s, ref) {
		t.Error("access after the stale 5G setup failure = 5GS, want EPS")
	}
}

// A gNB that cannot set up the resource will not bind, so the move onto 5GS is
// dropped and the UE can ask for it again.
func TestSetupFailureAbandonsATransferOnto5GS(t *testing.T) {
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

	n2Fail, err := buildPDUSessionResourceSetupUnsuccessfulTransferForTest()
	if err != nil {
		t.Fatalf("build unsuccessful transfer: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupFail(ctx, bearer.Ref, n2Fail); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupFail: %v", err)
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Fatal("the PDN connection was released by a gNB setup failure")
	}

	if !onEPS(t, s, bearer.Ref) {
		t.Error("access after the gNB setup failure = 5GS, want EPS")
	}

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil {
		t.Errorf("CreateSmContext(existing PDU session) after the abandoned leg = %v, want it accepted", err)
	}
}

// movedFromEPSTo5GS establishes a PDN connection and moves it to 5GS, binding
// the gNB so the move commits. It returns the session's ref, which the move
// preserves.
func movedFromEPSTo5GS(t *testing.T, s *smf.SMF) string {
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

	bindGNB(t, s, bearer.Ref)

	return bearer.Ref
}

// Network rules are subscriber and policy scoped, and the QER's QFI keys the
// UPF's downlink-notification state, so neither is given up with the access.
func TestTransferKeepsSubscriberScopedPolicy(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()

	rules := []*smf.ResolvedNetworkRule{{Description: "allow-all", PolicyID: "p1", Precedence: 10}}

	pcf.mu.Lock()
	pcf.policy.NetworkRules = rules
	qfi := pcf.policy.QosData.QFI
	pcf.mu.Unlock()

	s := newTestSMF(pcf, store, upf, amfCb)
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

	sc := s.GetSession(ref)

	defer sc.LockForTest()()

	if len(sc.PolicyData.NetworkRules) != len(rules) {
		t.Errorf("network rules after the move = %d, want the subscriber's %d", len(sc.PolicyData.NetworkRules), len(rules))
	}

	if sc.PolicyData.QosData.QFI != qfi {
		t.Errorf("QFI after the move = %d, want it stable at %d", sc.PolicyData.QosData.QFI, qfi)
	}

	if sc.Tunnel.DataPath.DownLinkTunnel.PDR.QER.QFI != qfi {
		t.Errorf("downlink QER QFI after the move = %d, want it stable at %d", sc.Tunnel.DataPath.DownLinkTunnel.PDR.QER.QFI, qfi)
	}
}

// TS 24.501 §6.5.3: a #47 naming the establishment accept's PTI means the UE
// never took the session. The accept assigns that PTI after the request, so the
// procedures the session left on EPS are discarded at the request and not at the
// commit that follows it.
func TestTransferOnto5GSKeepsTheEstablishmentPTI(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref := movedFromEPSTo5GS(t, s)

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, build5GSMStatus(transferTestPDUSessionID, 10, fgs.GSMCausePTIMismatch)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg(5GSM STATUS): %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Error("session retained after a 5GSM STATUS #47 named the transferred session's establishment PTI, want it released")
	}
}

// gatedUPF holds one session modification open, so a caller can act while the
// bind that issued it is in flight.
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

	return g
}

// gateNextApply holds the next statement to the UPF open, so a second procedure
// can reach the session while the first is mid-apply.
func (g *gatedUPF) gateNextApply() {
	g.arm <- struct{}{}
}

func (g *gatedUPF) Apply(ctx context.Context, desired *models.SessionState) (*models.SessionApplied, error) {
	select {
	case <-g.arm:
		close(g.entered)
		<-g.proceed
	default:
	}

	return g.fakeUPF.Apply(ctx, desired)
}

// A release and a bind both rewrite the session across a blocking UPF call, so
// one has to lose. The session is left on exactly one access, whichever wins.
func TestReleaseRacingATransferCommitLeavesOneOutcome(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	gated := newGatedUPF(upf)
	s := newTestSMF(pcf, store, gated, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	gated.gateNextApply()

	bound := make(chan error, 1)

	go func() { bound <- s.ModifyEPSSession(ctx, ref, enbFTEID()) }()

	<-gated.entered

	released := make(chan error, 1)

	go func() { released <- s.ReleaseSmContext(ctx, ref) }()

	close(gated.proceed)

	bindErr := <-bound
	releaseErr := <-released

	sc := s.GetSession(ref)

	switch {
	case bindErr == nil && releaseErr == nil:
		t.Error("the bind and the release both committed against the same session")
	case bindErr == nil:
		if sc == nil {
			t.Fatal("the session is gone after a committed bind and a refused release")
		}

		if !onEPS(t, s, ref) {
			t.Error("session is on 5GS after a committed bind onto EPS")
		}
	case releaseErr == nil:
		if sc != nil {
			t.Error("the session is still in the pool after a committed release")
		}
	default:
		t.Fatalf("both procedures failed: bind %v, release %v; want exactly one to commit", bindErr, releaseErr)
	}
}

// A release on the access serving the session ends the transfer too, and the
// access it was moving onto holds state from the accept it already sent.
func TestReleaseOnTheServingAccessTellsTheAbandonedTarget(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	// The MME holds the bearer from here; the eNB never binds it.
	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	if err := s.ReleaseSmContext(ctx, ref); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("the session survived a release on the access serving it")
	}

	if released := mmeCb.releasedAway(); len(released) != 1 || released[0] != epsTestEBI {
		t.Errorf("MME told of released connections = %v, want [%d]: it holds the bearer it was handed", released, epsTestEBI)
	}
}
