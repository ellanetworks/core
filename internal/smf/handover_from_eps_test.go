// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	libngap "github.com/ellanetworks/core/ngap"
)

const (
	arrivingPDUSessionID uint8  = 3
	targetGnbTEID        uint32 = 0x8001
)

var (
	sourceENB     = models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	targetGnbIPv4 = net.ParseIP("10.3.0.44").To4()
)

func establishEPSForArrival(t *testing.T, s *smf.SMF) *smf.SMContext {
	t.Helper()

	ctx := context.Background()

	req := epsRequest(3)
	req.APN = testDNN
	req.PDUSessionID = arrivingPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if err := s.ModifyEPSSession(ctx, bearer.Ref, epsTestEBI, sourceENB); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	return sc
}

func prepareArrival(t *testing.T, s *smf.SMF, upf *fakeUPF) (sc *smf.SMContext, ref string, n2 []byte, pfcpBefore int) {
	t.Helper()

	sc = establishEPSForArrival(t, s)
	pfcpBefore = modifyCount(upf)

	ref, n2, err := s.PrepareSmContextFromEPS(context.Background(), testSUPI(),
		arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	if ref != sc.Ref {
		t.Fatalf("prepare returned ref %q, want the session's own %q", ref, sc.Ref)
	}

	return sc, ref, n2, pfcpBefore
}

func accessOf(t *testing.T, sc *smf.SMContext) smf.AccessType {
	t.Helper()

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	return sc.Access
}

func modifyCount(f *fakeUPF) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.modifyCalls)
}

func TestPrepareSmContextFromEPSReportsTheAnchorTunnel(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc, _, n2, pfcpBefore := prepareArrival(t, s, upf)
	before := invariantsOf(t, sc)

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(n2)
	if err != nil {
		t.Fatalf("parse the Handover Request transfer: %v", err)
	}

	if got := uint32(transfer.ULNGUUPTNLInformation.GTPTunnel.GTPTEID); got != before.n3TEID {
		t.Errorf("uplink N3 TEID = %#x, want the anchor's %#x", got, before.n3TEID)
	}

	if transfer.DataForwardingNotPossible == nil {
		t.Error("the target NG-RAN node was not told that data forwarding is impossible")
	}

	if len(transfer.QosFlowSetupRequest) != 1 {
		t.Fatalf("QoS flows to set up = %d, want 1", len(transfer.QosFlowSetupRequest))
	}

	if access := accessOf(t, sc); access != smf.Access4G {
		t.Errorf("session moved to %s at preparation; the UE is still on E-UTRAN", access)
	}

	if got := modifyCount(upf) - pfcpBefore; got != 0 {
		t.Errorf("PFCP modifications during preparation = %d, want 0", got)
	}
}

func TestHandoverFromEPSKeepsTheSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc, ref, _, pfcpBefore := prepareArrival(t, s, upf)
	before := invariantsOf(t, sc)
	establishes := len(store.ops())

	ack, err := buildHandoverRequestAcknowledgeTransfer(targetGnbTEID, targetGnbIPv4)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if access := accessOf(t, sc); access != smf.Access4G {
		t.Errorf("session moved to %s before the UE arrived", access)
	}

	if got := modifyCount(upf) - pfcpBefore; got != 0 {
		t.Errorf("PFCP modifications before the UE arrived = %d, want 0", got)
	}

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	if after := invariantsOf(t, sc); after != before {
		t.Errorf("session invariants changed across the handover: %+v, want %+v", after, before)
	}

	sc.Mutex.Lock()
	access, ebi := sc.Access, sc.EBI
	dl := sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation
	sc.Mutex.Unlock()

	if access != smf.Access5G {
		t.Errorf("session is on %s after the UE arrived on the target NG-RAN node", access)
	}

	if ebi != epsTestEBI {
		t.Errorf("session EPS bearer identity = %d, want the %d it arrived with: a later handover to EPS needs it", ebi, epsTestEBI)
	}

	if dl.TEID != targetGnbTEID || !dl.IPv4Address.Equal(targetGnbIPv4) {
		t.Errorf("downlink points at %s/%#x, want the target gNB %s/%#x", dl.IPv4Address, dl.TEID, targetGnbIPv4, targetGnbTEID)
	}

	if dl.S1U {
		t.Error("the downlink is still addressed as S1-U after the session moved onto 5GS")
	}

	if got := modifyCount(upf) - pfcpBefore; got != 1 {
		t.Errorf("PFCP modifications on arrival = %d, want 1", got)
	}

	if s.SessionCount() != 1 {
		t.Errorf("sessions = %d, want 1: the handover must not create a second", s.SessionCount())
	}

	if got := len(store.ops()); got != establishes {
		t.Errorf("IP pool operations = %d, want %d: the handover must not touch the lease", got, establishes)
	}

	calls := mmeCb.dropped()
	if len(calls) != 1 {
		t.Fatalf("MME SessionDropped calls = %d, want 1", len(calls))
	}

	if calls[0].ref != sc.Ref || calls[0].ebi != epsTestEBI {
		t.Errorf("MME told about ref %q ebi %d, want %q / %d", calls[0].ref, calls[0].ebi, sc.Ref, epsTestEBI)
	}

	if s.GetSession(sc.Ref) == nil {
		t.Error("the session was released by the handover")
	}
}

func TestRefusedArrivalLeavesThePDNConnectionOnEPS(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc, ref, _, pfcpBefore := prepareArrival(t, s, upf)

	refused, err := (&libngap.HandoverResourceAllocationUnsuccessfulTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkRadioResourcesNotAvailable},
	}).Marshal()
	if err != nil {
		t.Fatalf("build the refusal transfer: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverFailed(ctx, ref, refused); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverFailed: %v", err)
	}

	if access := accessOf(t, sc); access != smf.Access4G {
		t.Errorf("session is on %s after the target refused it", access)
	}

	if got := modifyCount(upf) - pfcpBefore; got != 0 {
		t.Errorf("PFCP modifications after a refusal = %d, want 0", got)
	}

	if _, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai); err != nil {
		t.Errorf("the refused move was not unwound, so a second attempt is refused: %v", err)
	}
}

func TestCanceledArrivalRestoresTheSourceENBDownlink(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc, ref, _, _ := prepareArrival(t, s, upf)

	ack, err := buildHandoverRequestAcknowledgeTransfer(targetGnbTEID, targetGnbIPv4)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverCanceled(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverCanceled: %v", err)
	}

	sc.Mutex.Lock()
	access := sc.Access
	dl := sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Errorf("session is on %s after the handover was cancelled", access)
	}

	if dl.TEID != sourceENB.TEID || dl.IPv4Address.String() != sourceENB.Addr.String() {
		t.Errorf("downlink points at %s/%#x, want the source eNB %s/%#x", dl.IPv4Address, dl.TEID, sourceENB.Addr, sourceENB.TEID)
	}

	if !dl.S1U {
		t.Error("the restored downlink is not addressed as S1-U, so the UPF would send it on the N3 endpoint")
	}

	if _, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai); err != nil {
		t.Errorf("the cancelled move was not unwound, so a second attempt is refused: %v", err)
	}
}

func TestPrepareSmContextFromEPSRefusesAMismatchedBearerIdentity(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establishEPSForArrival(t, s)

	if _, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI+1, testDNN, testSnssai); err == nil {
		t.Fatal("a PDN connection was moved under an EPS bearer identity the anchor does not hold")
	}

	if access := accessOf(t, sc); access != smf.Access4G {
		t.Errorf("session is on %s after a refused preparation", access)
	}

	if _, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai); err != nil {
		t.Errorf("the refused preparation was not unwound: %v", err)
	}
}

func TestAbandonedArrivalPutsTheDownlinkBackOnTheSourceENB(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	defer smf.SetTransferSupervisionForTest(20 * time.Millisecond)()

	ctx := context.Background()

	sc := establishEPSForArrival(t, s)

	ref, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	ack, err := buildHandoverRequestAcknowledgeTransfer(targetGnbTEID, targetGnbIPv4)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sc.Mutex.Lock()
		pending := sc.TransferPendingForTest()
		sc.Mutex.Unlock()

		if !pending {
			break
		}

		time.Sleep(5 * time.Millisecond)
	}

	sc.Mutex.Lock()
	pending, access := sc.TransferPendingForTest(), sc.Access
	dl := sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation
	sc.Mutex.Unlock()

	if pending {
		t.Fatal("the arrival was never abandoned")
	}

	if access != smf.Access4G {
		t.Errorf("session is on %s after the arrival was abandoned", access)
	}

	if dl.TEID != sourceENB.TEID || !dl.S1U {
		t.Errorf("downlink points at %s/%#x s1u=%t, want the source eNB %s/%#x s1u=true",
			dl.IPv4Address, dl.TEID, dl.S1U, sourceENB.Addr, sourceENB.TEID)
	}
}

// TS 23.502 §4.11.1.2.3 step 4b
func TestReleasingAPreparedArrivalRestoresTheSourceENBDownlink(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc, ref, _, _ := prepareArrival(t, s, upf)

	ack, err := buildHandoverRequestAcknowledgeTransfer(targetGnbTEID, targetGnbIPv4)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if err := s.ReleaseSmContext(ctx, ref); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	sc.Mutex.Lock()
	access := sc.Access
	dl := sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Errorf("session is on %s after the arrival was released", access)
	}

	if dl.TEID != sourceENB.TEID || dl.IPv4Address.String() != sourceENB.Addr.String() {
		t.Errorf("downlink points at %s/%#x, want the source eNB %s/%#x", dl.IPv4Address, dl.TEID, sourceENB.Addr, sourceENB.TEID)
	}

	if !dl.S1U {
		t.Error("the restored downlink is not addressed as S1-U, so the UPF would GTP-U it to a gNB the UE never reached")
	}
}

func TestAFailedCompletionKeepsTheRollbackAnchor(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc, ref, _, _ := prepareArrival(t, s, upf)

	ack, err := buildHandoverRequestAcknowledgeTransfer(targetGnbTEID, targetGnbIPv4)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	sc.Mutex.Lock()
	sc.Tunnel = nil
	sc.Mutex.Unlock()

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err == nil {
		t.Fatal("a completion with no tunnel was accepted")
	}

	sc.Mutex.Lock()
	stranded := sc.HandoverSourceANForTest() == nil
	sc.Mutex.Unlock()

	if stranded {
		t.Error("the rollback anchor was discarded on a failed completion, so the cancel that follows restores nothing")
	}
}

// TS 23.502 §4.11.1.2.2.2 step 7
func TestArrivalTransferCarriesTheEBIToQFIMapping(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	_, _, n2, _ := prepareArrival(t, s, upf) //nolint:dogsled // only the N2 transfer is under test

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(n2)
	if err != nil {
		t.Fatalf("parse the arrival transfer: %v", err)
	}

	if len(transfer.QosFlowSetupRequest) != 1 {
		t.Fatalf("QoS flows = %d, want 1", len(transfer.QosFlowSetupRequest))
	}

	got := transfer.QosFlowSetupRequest[0].ERABID
	if got == nil {
		t.Fatal("no E-RAB ID, so the target cannot map the QoS flow back to its EPS bearer")
	}

	if uint8(*got) != epsTestEBI {
		t.Errorf("E-RAB ID = %d, want the arriving EPS bearer identity %d", *got, epsTestEBI)
	}
}

func TestArrivalAdmittedUnderAForeignQFIIsRefused(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc, ref, _, _ := prepareArrival(t, s, upf)

	ack, err := buildHandoverRequestAcknowledgeTransferWithQFI(targetGnbTEID, targetGnbIPv4, models.DefaultQFI+1)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err == nil {
		t.Fatal("the target admitted a QoS flow the SMF never asked for, and the arrival was accepted")
	}

	sc.Mutex.Lock()
	access := sc.Access
	stranded := sc.HandoverSourceANForTest()
	dl := sc.Tunnel.DownlinkPDR.FAR.ForwardingParameters.OuterHeaderCreation
	sc.Mutex.Unlock()

	if access != smf.Access4G {
		t.Errorf("session moved to %s on a refused admission", access)
	}

	if stranded != nil {
		t.Error("a rollback anchor was captured for an admission that was refused")
	}

	if dl.TEID != sourceENB.TEID || !dl.S1U {
		t.Errorf("downlink points at %s/%#x, want the source eNB", dl.IPv4Address, dl.TEID)
	}
}
