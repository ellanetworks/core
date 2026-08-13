// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func establishEPSOnENB(t *testing.T, s *smf.SMF) *smf.SMContext {
	t.Helper()

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
	if err := s.ModifyEPSSession(ctx, bearer.Ref, epsTestEBI, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	return sc
}

// TS 29.244 §8.2.26
func assertDownlinkBuffers(t *testing.T, sc *smf.SMContext) {
	t.Helper()

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	far := sc.Tunnel.DownlinkPDR.FAR

	if want := (models.ApplyAction{Buff: true, Nocp: true}); far.ApplyAction != want {
		t.Errorf("downlink apply action = %+v, want %+v", far.ApplyAction, want)
	}

	if far.ForwardingParameters != nil && far.ForwardingParameters.OuterHeaderCreation != nil {
		t.Errorf("downlink still aims at %s, want no outer header: the UE left that access",
			ohcSummary(far.ForwardingParameters.OuterHeaderCreation))
	}
}

func accessAndEBI(t *testing.T, sc *smf.SMContext) (smf.AccessType, uint8) {
	t.Helper()

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	return sc.Access, sc.EBI
}

// TS 23.401 §5.3.3.1
func TestTransferIdle5GSToEPSKeepsTheSessionWithTheUserPlaneDown(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establish5GS(t, s)
	before := invariantsOf(t, sc)
	establishes := len(store.ops())

	ref, err := s.TransferIdle(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access4G)
	if err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	if ref != sc.Ref {
		t.Errorf("move returned ref %q, want the session's own %q", ref, sc.Ref)
	}

	access, ebi := accessAndEBI(t, sc)
	if access != smf.Access4G {
		t.Errorf("session on %s after the move, want EPS", access)
	}

	if ebi != epsTestEBI {
		t.Errorf("session EBI = %d, want the %d the MME allocated", ebi, epsTestEBI)
	}

	if after := invariantsOf(t, sc); after != before {
		t.Errorf("session invariants changed across the move: %+v, want %+v", after, before)
	}

	if got := len(store.ops()); got != establishes {
		t.Errorf("IP pool operations = %d, want %d: the move must not touch the lease", got, establishes)
	}

	assertDownlinkBuffers(t, sc)
	assertMovable(t, sc)

	calls := amfCb.dropped()
	if len(calls) != 1 {
		t.Fatalf("AMF SessionDropped calls = %d, want 1", len(calls))
	}

	if calls[0].ref != sc.Ref || calls[0].pduSessionID != movedPDUSessionID {
		t.Errorf("AMF told about ref %q pdu session %d, want %q / %d",
			calls[0].ref, calls[0].pduSessionID, sc.Ref, movedPDUSessionID)
	}
}

// TS 24.501 §5.5.1.3.2
func TestTransferIdleEPSTo5GSKeepsTheSessionWithTheUserPlaneDown(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establishEPSOnENB(t, s)
	before := invariantsOf(t, sc)
	establishes := len(store.ops())

	ref, err := s.TransferIdle(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access5G)
	if err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	if ref != sc.Ref {
		t.Errorf("move returned ref %q, want the session's own %q", ref, sc.Ref)
	}

	access, ebi := accessAndEBI(t, sc)
	if access != smf.Access5G {
		t.Errorf("session on %s after the move, want 5GS", access)
	}

	if ebi != epsTestEBI {
		t.Errorf("session EBI = %d, want the mapped %d it keeps on 5GS", ebi, epsTestEBI)
	}

	if after := invariantsOf(t, sc); after != before {
		t.Errorf("session invariants changed across the move: %+v, want %+v", after, before)
	}

	if got := len(store.ops()); got != establishes {
		t.Errorf("IP pool operations = %d, want %d: the move must not touch the lease", got, establishes)
	}

	assertDownlinkBuffers(t, sc)
	assertMovable(t, sc)

	calls := mmeCb.dropped()
	if len(calls) != 1 {
		t.Fatalf("MME SessionDropped calls = %d, want 1", len(calls))
	}

	if calls[0].ref != sc.Ref || calls[0].ebi != epsTestEBI {
		t.Errorf("MME told about ref %q ebi %d, want %q / %d", calls[0].ref, calls[0].ebi, sc.Ref, epsTestEBI)
	}
}

func TestTransferIdleLeavesNoMoveForTheGuardToAbandon(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	defer smf.SetTransferSupervisionForTest(20 * time.Millisecond)()

	ctx := context.Background()
	sc := establish5GS(t, s)

	if _, err := s.TransferIdle(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access4G); err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	access, _ := accessAndEBI(t, sc)
	if access != smf.Access4G {
		t.Errorf("session on %s once the supervision window passed, want EPS: the move was committed, not staged", access)
	}

	assertMovable(t, sc)

	if s.GetSession(sc.Ref) == nil {
		t.Error("the session was released after the move committed")
	}
}

func TestTransferIdleRestoresTheSourceWhenTheModifyFails(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establishEPSOnENB(t, s)

	sc.Mutex.Lock()
	before := ohcSummary(sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation)
	beforeAction := sc.Tunnel.DownlinkPDR.FAR.ApplyAction
	sc.Mutex.Unlock()

	upf.err = errors.New("PFCP modify refused")

	if _, err := s.TransferIdle(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access5G); err == nil {
		t.Fatal("the move succeeded though the UPF refused the modify")
	}

	upf.err = nil

	access, ebi := accessAndEBI(t, sc)
	if access != smf.Access4G {
		t.Errorf("session on %s after a refused move, want EPS", access)
	}

	if ebi != epsTestEBI {
		t.Errorf("session EBI = %d after a refused move, want %d", ebi, epsTestEBI)
	}

	sc.Mutex.Lock()
	after := ohcSummary(sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation)
	afterAction := sc.Tunnel.DownlinkPDR.FAR.ApplyAction
	sc.Mutex.Unlock()

	if after != before {
		t.Errorf("downlink outer header = %s, want the eNB's %s: the data plane still holds it", after, before)
	}

	if afterAction != beforeAction {
		t.Errorf("downlink apply action = %+v, want %+v", afterAction, beforeAction)
	}

	assertMovable(t, sc)

	if len(mmeCb.dropped()) != 0 {
		t.Error("the MME was told to drop a session that never left EPS")
	}
}

// TS 23.401 §5.3.3.1
func TestTransferIdleTo4GLetsTheENBBindTheDownlink(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establish5GS(t, s)

	ref, err := s.TransferIdle(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access4G)
	if err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, ref, epsTestEBI, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc.Mutex.Lock()
	far := sc.Tunnel.DownlinkPDR.FAR
	forwarding := far.ApplyAction.Forw
	ohc := far.ForwardingParameters.OuterHeaderCreation
	sc.Mutex.Unlock()

	if !forwarding {
		t.Error("the downlink still buffers after the eNB bound it")
	}

	if ohc == nil || ohc.TEID != enb.TEID {
		t.Errorf("downlink outer header = %s, want the eNB's TEID %#x", ohcSummary(ohc), enb.TEID)
	}
}

func TestTransferIdleTo5GSLetsTheGNBBindTheDownlink(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establishEPSOnENB(t, s)

	ref, err := s.TransferIdle(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access5G)
	if err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	if _, err := s.ActivateSmContext(ctx, ref); err != nil {
		t.Fatalf("ActivateSmContext: %v", err)
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	sc.Mutex.Lock()
	far := sc.Tunnel.DownlinkPDR.FAR
	forwarding := far.ApplyAction.Forw
	ohc := far.ForwardingParameters.OuterHeaderCreation
	sc.Mutex.Unlock()

	if !forwarding {
		t.Error("the downlink still buffers after the gNB bound it")
	}

	if ohc == nil || ohc.TEID != 0x7001 {
		t.Errorf("downlink outer header = %s, want the gNB's TEID 0x7001", ohcSummary(ohc))
	}
}
