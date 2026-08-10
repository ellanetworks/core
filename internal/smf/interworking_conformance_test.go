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
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

var errAnchorModifyRefused = errors.New("anchor refused the session modification")

// Conformance tests for 4G/5G interworking without N26

func movedOnEPS(t *testing.T, s *smf.SMF, pdnType uint8) models.EPSBearer {
	t.Helper()

	req := epsRequest(pdnType)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(context.Background(), bearer.Ref, epsTestEBI, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	return bearer
}

// TS 23.502 §4.3.2.2.1 step 8
func TestInterworkingExistingPDUSessionKeepsTheUEAddress(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	store.allocatedIPv6 = netip.MustParseAddr("2001:db8:2::")

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	bearer := movedOnEPS(t, s, 3)

	before := len(amfCb.n1n2())

	if _, reject, err := s.CreateSmContext(context.Background(), testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	calls := amfCb.n1n2()
	if len(calls) != before+1 {
		t.Fatalf("N1N2 transfers = %d, want %d", len(calls), before+1)
	}

	accept := parseEstablishmentAccept(t, calls[len(calls)-1].n1Msg)
	if accept.PDUAddress == nil {
		t.Fatal("the establishment accept carries no PDU address")
	}

	if accept.PDUAddress.IPv6IID != bearer.IPv6IID {
		t.Errorf("interface identifier %x, want the %x held on EPS", accept.PDUAddress.IPv6IID, bearer.IPv6IID)
	}

	if s.SessionCount() != 1 {
		t.Errorf("sessions = %d, want 1: the move must not create a second", s.SessionCount())
	}
}

func TestInterworkingInitialRequestDoesNotKeepTheUEAddress(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	store.allocatedIPv6 = netip.MustParseAddr("2001:db8:2::")

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	bearer := movedOnEPS(t, s, 3)

	before := len(amfCb.n1n2())

	if _, reject, err := s.CreateSmContext(context.Background(), testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil || reject != nil {
		t.Fatalf("establish on 5GS: %v (reject %d bytes)", err, len(reject))
	}

	accept := parseEstablishmentAccept(t, amfCb.n1n2()[before].n1Msg)
	if accept.PDUAddress == nil {
		t.Fatal("the establishment accept carries no PDU address")
	}

	if accept.PDUAddress.IPv6IID == bearer.IPv6IID {
		t.Error("an initial request reused the EPS interface identifier, so the moved case cannot be told from the re-established one")
	}
}

// TS 23.502 §4.11.2.2
func TestInterworkingDeactivationFromTheOtherAccessIsANoOp(t *testing.T) {
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
		t.Error("EPS buffered the downlink of a session 5GS is serving over N3")
	}

	// The mirror: 5GS must not deactivate a session that has moved to EPS either.
	bearer := movedOnEPS(t, s, 1)

	modifies = len(upf.modifyCalls)

	if err := s.DeactivateSmContext(ctx, bearer.Ref); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if len(upf.modifyCalls) != modifies {
		t.Error("5GS buffered the downlink of a session EPS is serving over S1-U")
	}
}

// TS 24.301 §6.4.4.3
func TestInterworkingEPSReleaseIsIdempotent(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establish5GS(t, s)

	if _, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID)); err != nil {
		t.Fatalf("prepare the move to EPS: %v", err)
	}

	for i := 1; i <= 3; i++ {
		if err := s.ReleaseEPSSession(ctx, sc.Ref); err != nil {
			t.Fatalf("ReleaseEPSSession call %d: %v", i, err)
		}

		if s.GetSession(sc.Ref) == nil {
			t.Fatalf("call %d tore down the session 5GS is still serving, freeing the UE's address", i)
		}
	}
}

func TestInterworkingEPSReleaseStillReleasesItsOwnSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()
	bearer := movedOnEPS(t, s, 1)

	if err := s.ReleaseEPSSession(ctx, bearer.Ref); err != nil {
		t.Fatalf("ReleaseEPSSession: %v", err)
	}

	if s.GetSession(bearer.Ref) != nil {
		t.Fatal("an EPS session was not released by the access that owns it")
	}

	if err := s.ReleaseEPSSession(ctx, bearer.Ref); err != nil {
		t.Errorf("releasing an already-released session: %v, want nil", err)
	}
}

func TestInterworkingInitialRequestBeforeAMoveSupersedesTheSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	store.allocatedIPv6 = netip.MustParseAddr("2001:db8:2::")

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()
	bearer := movedOnEPS(t, s, 3)

	if _, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil || reject != nil {
		t.Fatalf("stray initial request: %v (reject %d bytes)", err, len(reject))
	}

	if s.GetSession(bearer.Ref) != nil {
		t.Error("the EPS session survived an initial request for the same PDU session identity")
	}

	// The move that follows can no longer find the session it meant to move.
	_, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err == nil {
		t.Fatal("a move succeeded against a session that had already been superseded")
	}

	assertGSMCause(t, reject, fgs.GSMCauseInsufficientResources)
}

// TS 23.502 §4.11.2.2 step 14
func TestInterworkingRefusedRANSetupDropsThe5GSRoutingContext(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()
	bearer := movedOnEPS(t, s, 1)

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	before := len(amfCb.dropped())

	fail, err := buildPDUSessionResourceSetupUnsuccessfulTransfer()
	if err != nil {
		t.Fatalf("build N2 setup failure: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupFail(ctx, ref, fail); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupFail: %v", err)
	}

	calls := amfCb.dropped()
	if len(calls) != before+1 {
		t.Fatalf("AMF SessionDropped calls = %d, want %d: its routing context outlives the move it was created for", len(calls), before+1)
	}

	if calls[len(calls)-1].n2Transfer != nil {
		t.Error("an N2 release went to a RAN that had just reported it has no resources for the session")
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Error("the session EPS is still serving was released by the failed move")
	}
}

func TestInterworking5GSReleaseSparesAPreparedEPSSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()
	bearer := movedOnEPS(t, s, 1)

	if _, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	if err := s.ReleaseSmContext(ctx, bearer.Ref); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Fatal("a 5GS release destroyed a PDN connection the MME is still serving, freeing the UE's address")
	}

	assertMovable(t, s.GetSession(bearer.Ref))
}

func TestInterworking5GSReleaseStillReleasesItsOwnSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()
	sc := establish5GS(t, s)

	if err := s.ReleaseSmContext(ctx, sc.Ref); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	if s.GetSession(sc.Ref) != nil {
		t.Fatal("a 5GS session was not released by the access that owns it")
	}

	if err := s.ReleaseSmContext(ctx, sc.Ref); err != nil {
		t.Errorf("releasing an already-released session: %v, want nil", err)
	}
}

// TS 24.501 §6.1.4.2
func TestInterworkingEstablishmentAcceptCarriesNoMappedEPSBearerContexts(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	establish5GS(t, s)

	calls := amfCb.n1n2()
	if len(calls) == 0 {
		t.Fatal("no establishment accept was sent")
	}

	const ieiMappedEPSBearerContext = 0x75

	for _, call := range calls {
		accept := parseEstablishmentAccept(t, call.n1Msg)

		for _, ie := range accept.Unrecognized {
			if ie.IEI == ieiMappedEPSBearerContext {
				t.Errorf("the establishment accept carries mapped EPS bearer contexts (IEI %#02x)", ie.IEI)
			}
		}
	}
}

// TS 24.501 §6.4.1.4.1
func TestInterworkingNoPoolAnswers5GSMCause28(t *testing.T) {
	for _, requested := range []fgs.PDUSessionType{
		fgs.PDUSessionTypeIPv4,
		fgs.PDUSessionTypeIPv6,
		fgs.PDUSessionTypeIPv4v6,
	} {
		t.Run(requested.String(), func(t *testing.T) {
			pcf, store, upf, amfCb, mmeCb := interworkingFakes()
			pcf.policy.IPv4Pool = ""
			pcf.policy.IPv6Pool = ""

			s := newTestSMF(pcf, store, upf, amfCb)
			s.SetMME(mmeCb)

			est := &fgs.PDUSessionEstablishmentRequest{
				PDUSessionID:             fgs.PDUSessionID(movedPDUSessionID),
				PTI:                      10,
				IntegrityProtMaxDataRate: [2]byte{0xff, 0xff},
				PDUSessionType:           &requested,
			}

			raw, err := est.MarshalBinary()
			if err != nil {
				t.Fatalf("build the establishment request: %v", err)
			}

			_, reject, err := s.CreateSmContext(context.Background(), testSUPI(), movedPDUSessionID, testDNN, testSnssai,
				fgs.RequestTypeInitialRequest, raw)
			if err == nil {
				t.Fatal("the establishment was accepted with no pool to draw an address from")
			}

			assertGSMCause(t, reject, fgs.GSMCauseUnknownPDUSessionType)
		})
	}
}

func TestInterworkingScrapeDoesNotBlockTeardown(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	for range 20 {
		establish5GS(t, s)
		removeSession(s, ctx, s.GetSession(establish5GS(t, s).Ref))
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := 0; i < 500; i++ {
			s.SessionCountByRAT()
		}
	}()

	for i := 0; i < 500; i++ {
		sc := establish5GS(t, s)
		removeSession(s, ctx, sc)
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("a scrape and a teardown deadlocked: the registry and session locks were taken in opposite orders")
	}
}

func TestInterworkingUncommittedMoveExpires(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	defer smf.SetTransferSupervisionForTest(20 * time.Millisecond)()

	ctx := context.Background()
	sc := establish5GS(t, s)

	if _, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID)); err != nil {
		t.Fatalf("prepare the move to EPS: %v", err)
	}

	sc.Mutex.Lock()
	pending := sc.TransferPendingForTest()
	sc.Mutex.Unlock()

	if !pending {
		t.Fatal("the move was not recorded")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sc.Mutex.Lock()
		pending = sc.TransferPendingForTest()
		sc.Mutex.Unlock()

		if !pending {
			break
		}

		time.Sleep(5 * time.Millisecond)
	}

	if pending {
		t.Fatal("the move was never abandoned; a RAN leg that goes silent leaves the session un-movable")
	}

	if s.GetSession(sc.Ref) == nil {
		t.Fatal("the session was released; only the move should expire")
	}

	assertMovable(t, sc)

	// And the UE can ask again.
	if _, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID)); err != nil {
		t.Errorf("the session is still un-movable after the move expired: %v", err)
	}
}

// TS 24.501 §6.4.1.7 e)
func TestInterworkingUndeliveredAcceptDrawsGSMCause26(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	movedOnEPS(t, s, 1)

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	before := len(amfCb.n1())

	fail, err := buildPDUSessionResourceSetupUnsuccessfulTransfer()
	if err != nil {
		t.Fatalf("build N2 setup failure: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupFail(ctx, ref, fail); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupFail: %v", err)
	}

	calls := amfCb.n1()
	if len(calls) != before+1 {
		t.Fatalf("N1 transfers = %d, want %d: the UE was left waiting on T3580 for an accept it never got", len(calls), before+1)
	}

	assertGSMCause(t, calls[len(calls)-1].n1Msg, fgs.GSMCauseInsufficientResources)
}

// TS 24.301 §6.5.1.4.1
func TestInterworkingRefusedPDNTypeNamesTheAllowedFamily(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ipv4Pool string
		ipv6Pool string
		want     eps.ESMCause
	}{
		{"both families", "10.0.0.0/24", "2001:db8::/32", eps.ESMCausePDNTypeIPv4v6OnlyAllowed},
		{"IPv4 only", "10.0.0.0/24", "", eps.ESMCausePDNTypeIPv4OnlyAllowed},
		{"IPv6 only", "", "2001:db8::/32", eps.ESMCausePDNTypeIPv6OnlyAllowed},
		{"neither", "", "", eps.ESMCauseInsufficientResources},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcf, store, upf, amfCb, mmeCb := interworkingFakes()
			s := newTestSMF(pcf, store, upf, amfCb)
			s.SetMME(mmeCb)

			req := epsRequest(uint8(eps.PDNTypeNonIP))
			req.APN = testDNN
			req.Snssai = testSnssai
			req.IPv4Pool = tc.ipv4Pool
			req.IPv6Pool = tc.ipv6Pool

			_, err := s.CreateEPSSession(context.Background(), req)

			var pdnErr *models.PDNTypeError
			if !errors.As(err, &pdnErr) {
				t.Fatalf("CreateEPSSession error = %v, want a PDN type error naming the allowed family", err)
			}

			if pdnErr.Cause != tc.want {
				t.Errorf("cause = %v, want %v", pdnErr.Cause, tc.want)
			}
		})
	}
}

// TS 24.501 §6.4.1.7 e)
func TestInterworkingReactivationFailureDoesNotRejectALiveSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	// establish5GS completes the N2 bind, so the establishment procedure is over.
	sc := establish5GS(t, s)

	before := len(amfCb.n1())

	fail, err := buildPDUSessionResourceSetupUnsuccessfulTransfer()
	if err != nil {
		t.Fatalf("build N2 setup failure: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupFail(ctx, sc.Ref, fail); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupFail: %v", err)
	}

	if got := len(amfCb.n1()); got != before {
		t.Errorf("N1 transfers = %d, want %d: a reactivation failure rejected an established session", got, before)
	}

	if s.GetSession(sc.Ref) == nil {
		t.Error("the established session was released by a reactivation failure")
	}
}

func TestInterworkingExpiredMoveDropsThe5GSRoutingContext(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	defer smf.SetTransferSupervisionForTest(20 * time.Millisecond)()

	ctx := context.Background()
	bearer := movedOnEPS(t, s, 1)

	before := len(amfCb.dropped())

	if _, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest()); err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(amfCb.dropped()) == before {
		time.Sleep(5 * time.Millisecond)
	}

	calls := amfCb.dropped()
	if len(calls) != before+1 {
		t.Fatalf("AMF SessionDropped calls = %d, want %d: the routing context outlives the move that created it", len(calls), before+1)
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Error("the session EPS is still serving was released by the expiry")
	}
}

// TS 24.501 §6.4.1.4
func TestInterworkingFailedMoveTo5GSAnswersTheUEAndDropsTheRoutingContext(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	movedOnEPS(t, s, 3)

	ref, reject, err := s.CreateSmContext(ctx, testSUPI(), movedPDUSessionID, testDNN, testSnssai,
		fgs.RequestTypeExistingPDUSession, buildDualStackPDUSessionEstRequest())
	if err != nil || reject != nil {
		t.Fatalf("move to 5GS: %v (reject %d bytes)", err, len(reject))
	}

	before := len(amfCb.n1())

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	upf.err = errAnchorModifyRefused

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err == nil {
		t.Fatal("the anchor reported success though the UPF refused the modification")
	}

	calls := amfCb.n1()
	if len(calls) != before+1 {
		t.Fatalf("N1 transfers = %d, want %d: the UE already holds the establishment accept, so a move that cannot complete owes it an answer", len(calls), before+1)
	}

	assertGSMCause(t, calls[len(calls)-1].n1Msg, fgs.GSMCauseInsufficientResources)

	var droppedRef bool

	for _, d := range amfCb.dropped() {
		if d.ref == ref {
			droppedRef = true
		}
	}

	if !droppedRef {
		t.Error("the AMF still holds the routing context for a session that never reached 5GS, and nothing else reaps it")
	}
}

// TS 23.502 §4.9.1.3
func TestInterworkingHandoverHandlersSurviveAReleasedSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(s *smf.SMF, ref string) error
	}{
		{"N2 handover preparing", func(s *smf.SMF, ref string) error {
			required, err := (&libngap.HandoverRequiredTransfer{}).Marshal()
			if err != nil {
				return err
			}

			_, err = s.UpdateSmContextN2HandoverPreparing(context.Background(), ref, required)

			return err
		}},
		{"N2 handover complete", func(s *smf.SMF, ref string) error {
			return s.UpdateSmContextN2HandoverComplete(context.Background(), ref)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcf, store, upf, amfCb, mmeCb := interworkingFakes()
			s := newTestSMF(pcf, store, upf, amfCb)
			s.SetMME(mmeCb)

			sc := establish5GS(t, s)

			sc.Mutex.Lock()
			sc.Tunnel = nil
			sc.PolicyData = nil
			sc.Mutex.Unlock()

			if err := tc.call(s, sc.Ref); err == nil {
				t.Error("a handover against a session whose user plane is gone reported success")
			}
		})
	}
}
