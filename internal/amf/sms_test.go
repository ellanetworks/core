// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smsf"
	"github.com/ellanetworks/core/nas/fgs"
)

type fakeSMSHandler struct {
	mu        sync.Mutex
	allowed   bool
	err       error
	pending   bool
	uplinks   [][]byte
	reachable []string
	hold      chan struct{}
	held      chan struct{}
}

func (h *fakeSMSHandler) AllowedEach(_ context.Context, imsis []string) (map[string]bool, error) {
	h.mu.Lock()
	ok, err, hold, held := h.allowed, h.err, h.hold, h.held
	h.mu.Unlock()

	if hold != nil {
		close(held)
		<-hold
	}

	if err != nil {
		return nil, err
	}

	allowed := make(map[string]bool, len(imsis))
	for _, imsi := range imsis {
		allowed[imsi] = ok
	}

	return allowed, nil
}

func (h *fakeSMSHandler) holdDecision() (held, release chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.hold, h.held = make(chan struct{}), make(chan struct{})

	return h.held, h.hold
}

func (h *fakeSMSHandler) setAllowed(allowed bool, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.allowed, h.err = allowed, err
}

func (h *fakeSMSHandler) Uplink(_ context.Context, _ string, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.uplinks = append(h.uplinks, payload)
}

func (h *fakeSMSHandler) UEReachable(_ context.Context, imsi string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.reachable = append(h.reachable, imsi)
}

func (h *fakeSMSHandler) TransactionPending(string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.pending
}

func (h *fakeSMSHandler) setPending(p bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.pending = p
}

func (h *fakeSMSHandler) snapshot() (uplinks [][]byte, reachable []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return slices.Clone(h.uplinks), slices.Clone(h.reachable)
}

var smsTestTAI = models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}

func newSMSTestAMF(t *testing.T) (*amf.AMF, *fakeSMSHandler, *fakeNGAPSender) {
	t.Helper()

	a := amf.New(&fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}, nil, &fakeSmf{})
	a.ClearRadiosForTest()

	handler := &fakeSMSHandler{allowed: true}
	a.SMS = handler

	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(a)
	a.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{Tai: smsTestTAI}})

	return a, handler, sender
}

func newIdleSMSUE(t *testing.T, a *amf.AMF) *amf.UeContext {
	t.Helper()

	ue := buildServedTestUE(t, a, "001019756139901")
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(testGUTI(t))
	ue.AllocateRegistrationArea([]models.Tai{smsTestTAI})

	if !a.GrantSMSOverNAS(t.Context(), ue, true) {
		t.Fatal("precondition: SMS over NAS must be granted")
	}

	return ue
}

func connectSMSUE(t *testing.T, a *amf.AMF, ue *amf.UeContext, sender *fakeNGAPSender) {
	t.Helper()

	radio, ok := a.RadioForTest(sender)
	if !ok {
		t.Fatal("precondition: the test radio must be registered")
	}

	a.AttachUeConn(t.Context(), ue, amf.NewUeConnForTest(radio, 1, 1))
}

func TestSMSOverNASIsGrantedOnlyWhenRequestedAndAllowed(t *testing.T) {
	cases := []struct {
		name      string
		requested bool
		allowed   bool
		handler   bool
		want      bool
	}{
		{name: "requested and allowed", requested: true, allowed: true, handler: true, want: true},
		{name: "not requested", requested: false, allowed: true, handler: true},
		{name: "not allowed", requested: true, allowed: false, handler: true},
		{name: "no SMSF", requested: true, allowed: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := amf.New(nil, nil, nil)
			if tc.handler {
				a.SMS = &fakeSMSHandler{allowed: tc.allowed}
			}

			ue := buildServedTestUE(t, a, "001019756139901")

			if got := a.GrantSMSOverNAS(t.Context(), ue, tc.requested); got != tc.want || ue.SMSOverNAS() != tc.want {
				t.Fatalf("granted = %t, UE context = %t, want %t", got, ue.SMSOverNAS(), tc.want)
			}
		})
	}
}

func TestRegistrationAcceptIndicatesWhetherSMSOverNASIsAllowed(t *testing.T) {
	for _, granted := range []bool{true, false} {
		a := amf.New(nil, nil, nil)
		a.SMS = &fakeSMSHandler{allowed: true}

		ue := buildServedTestUE(t, a, "001019756139901")
		a.GrantSMSOverNAS(t.Context(), ue, granted)

		raw, err := amf.BuildRegistrationAccept(a, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
		if err != nil {
			t.Fatalf("BuildRegistrationAccept: %v", err)
		}

		ra, err := fgs.ParseRegistrationAccept(raw)
		if err != nil {
			t.Fatalf("parse RegistrationAccept: %v", err)
		}

		if ra.SMSAllowed != granted {
			t.Fatalf("SMS allowed = %t, want %t", ra.SMSAllowed, granted)
		}
	}
}

func TestReachabilityForSMSIsRefusedForAUEThatCannotReceiveIt(t *testing.T) {
	a, _, _ := newSMSTestAMF(t)

	if err := a.EnableUEReachabilityForSMS(t.Context(), "001019756139999"); !errors.Is(err, smsf.ErrNotRegisteredForSMS) {
		t.Fatalf("unknown UE: err = %v, want %v", err, smsf.ErrNotRegisteredForSMS)
	}

	ue := newIdleSMSUE(t, a)
	a.GrantSMSOverNAS(t.Context(), ue, false)

	if err := a.EnableUEReachabilityForSMS(t.Context(), "001019756139901"); !errors.Is(err, smsf.ErrNotRegisteredForSMS) {
		t.Fatalf("UE without SMS over NAS: err = %v, want %v", err, smsf.ErrNotRegisteredForSMS)
	}

	ue.ForceStateForTest(amf.Deregistered)

	if err := a.EnableUEReachabilityForSMS(t.Context(), "001019756139901"); !errors.Is(err, smsf.ErrNotRegisteredForSMS) {
		t.Fatalf("deregistered UE: err = %v, want %v", err, smsf.ErrNotRegisteredForSMS)
	}
}

func TestAConnectedUEIsReachableForSMSWithoutPaging(t *testing.T) {
	a, _, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)

	if err := a.EnableUEReachabilityForSMS(t.Context(), "001019756139901"); err != nil {
		t.Fatalf("EnableUEReachabilityForSMS: %v", err)
	}

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0", sender.pagingCalls)
	}
}

func TestAnIdleUEIsPagedAndReachableOnceTheProcedureSettles(t *testing.T) {
	a, _, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)

	done := make(chan error, 1)

	go func() { done <- a.EnableUEReachabilityForSMS(context.Background(), "001019756139901") }()

	waitFor(t, func() bool { return ue.PagingState() == amf.PagingAttempting })

	connectSMSUE(t, a, ue, sender)

	if ue.PagingState() != amf.PagingDelivering {
		t.Fatalf("paging state = %s, want Delivering", ue.PagingState())
	}

	select {
	case err := <-done:
		t.Fatalf("reachability reported (%v) before the paging procedure settled", err)
	case <-time.After(50 * time.Millisecond):
	}

	ue.PagingDelivered(t.Context())

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("EnableUEReachabilityForSMS: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reachability not reported after the paging procedure settled")
	}

	if sender.pagingCalls != 1 {
		t.Fatalf("paging calls = %d, want 1", sender.pagingCalls)
	}
}

func TestAnUnansweredPageMakesTheUEUnreachableForSMS(t *testing.T) {
	a, _, _ := newSMSTestAMF(t)
	a.T3513Cfg = guard.TimerValue{Enable: true, ExpireTime: 20 * time.Millisecond, MaxRetryTimes: 1}
	newIdleSMSUE(t, a)

	if err := a.EnableUEReachabilityForSMS(t.Context(), "001019756139901"); !errors.Is(err, smsf.ErrUnreachable) {
		t.Fatalf("err = %v, want %v", err, smsf.ErrUnreachable)
	}
}

func TestReachabilityForSMSStopsWaitingAtTheDeadline(t *testing.T) {
	a, _, _ := newSMSTestAMF(t)
	a.T3513Cfg = guard.TimerValue{Enable: true, ExpireTime: time.Hour, MaxRetryTimes: 1}
	ue := newIdleSMSUE(t, a)

	defer ue.StopPagingForTest(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if err := a.EnableUEReachabilityForSMS(ctx, "001019756139901"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want %v", err, context.DeadlineExceeded)
	}
}

func TestSMSIsSentOnlyOverAnExistingConnection(t *testing.T) {
	a, _, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)

	if err := a.SendSMS(t.Context(), "001019756139901", []byte{0x09, 0x04}); !errors.Is(err, smsf.ErrUnreachable) {
		t.Fatalf("idle UE: err = %v, want %v", err, smsf.ErrUnreachable)
	}

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0", sender.pagingCalls)
	}

	connectSMSUE(t, a, ue, sender)

	if err := a.SendSMS(t.Context(), "001019756139901", []byte{0x09, 0x04}); err != nil {
		t.Fatalf("connected UE: %v", err)
	}

	if sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("downlink NAS transports = %d, want 1", sender.downlinkNasTransportCalls)
	}
}

func TestUplinkSMSIsForwardedOnlyForAUEWithSMSOverNAS(t *testing.T) {
	a, handler, _ := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)

	a.ForwardSMS(t.Context(), ue, []byte{0x09, 0x01})
	a.GrantSMSOverNAS(t.Context(), ue, false)
	a.ForwardSMS(t.Context(), ue, []byte{0x09, 0x02})

	uplinks, _ := handler.snapshot()
	if len(uplinks) != 1 || uplinks[0][1] != 0x01 {
		t.Fatalf("uplinks = %x, want only the one sent while SMS over NAS was allowed", uplinks)
	}
}

func TestCompletingARegistrationReportsTheUEReachableForSMS(t *testing.T) {
	a, handler, _ := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	a.MarkRegistered(t.Context(), ue)

	a.GrantSMSOverNAS(t.Context(), ue, false)
	ue.ForceStateForTest(amf.RegistrationInitiated)
	a.MarkRegistered(t.Context(), ue)

	if _, reachable := handler.snapshot(); !slices.Equal(reachable, []string{"001019756139901"}) {
		t.Fatalf("reachable = %v, want one report while SMS over NAS was allowed", reachable)
	}
}

func TestDeregistrationDeregistersTheUEFromSMS(t *testing.T) {
	a, _, _ := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)

	ue.Deregister(t.Context())

	if ue.SMSOverNAS() {
		t.Fatal("the grant survived the deregistration")
	}

	if granted, _ := a.SMSRoute("001019756139901"); granted {
		t.Fatal("the AMF still routes SMS to a deregistered UE")
	}
}

func TestRemovingTheUEContextDeregistersItFromSMS(t *testing.T) {
	a, _, _ := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)

	a.DeregisterAndRemoveUeContext(t.Context(), ue)

	if granted, _ := a.SMSRoute("001019756139901"); granted {
		t.Fatal("the AMF still routes SMS to a removed UE")
	}
}

func TestAnOpenSMSTransactionHoldsTheConnection(t *testing.T) {
	a, handler, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)

	if ue.Conn().MTSignallingPending() {
		t.Fatal("signalling pending with no SMS transaction")
	}

	handler.setPending(true)

	if !ue.Conn().MTSignallingPending() {
		t.Fatal("an open SMS transaction does not hold the connection")
	}
}

func TestReachabilityForSMSWaitsForARegistrationInProgress(t *testing.T) {
	a, _, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	done := make(chan error, 1)

	go func() { done <- a.EnableUEReachabilityForSMS(context.Background(), "001019756139901") }()

	select {
	case err := <-done:
		t.Fatalf("reachability reported (%v) during the registration", err)
	case <-time.After(50 * time.Millisecond):
	}

	a.MarkRegistered(t.Context(), ue)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("EnableUEReachabilityForSMS: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reachability not reported after the registration completed")
	}
}

func TestReachabilityForSMSEndsWhenTheRegistrationIsAbandoned(t *testing.T) {
	a, _, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	done := make(chan error, 1)

	go func() { done <- a.EnableUEReachabilityForSMS(context.Background(), "001019756139901") }()

	time.Sleep(20 * time.Millisecond)
	ue.Deregister(t.Context())

	select {
	case err := <-done:
		if !errors.Is(err, smsf.ErrNotRegisteredForSMS) {
			t.Fatalf("err = %v, want %v", err, smsf.ErrNotRegisteredForSMS)
		}
	case <-time.After(time.Second):
		t.Fatal("reachability still waiting after the UE deregistered")
	}
}

func TestSMSOverNASIsNotAllowedWhenTheGrantCannotBeDecided(t *testing.T) {
	a, handler, _ := newSMSTestAMF(t)
	handler.setAllowed(true, errors.New("leader changed"))

	ue := buildServedTestUE(t, a, "001019756139901")

	if a.GrantSMSOverNAS(t.Context(), ue, true) {
		t.Fatal("SMS over NAS granted while the grant could not be decided")
	}
}

func TestSMSGrantFollowsConfigurationChanges(t *testing.T) {
	a, handler, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)

	handler.setAllowed(false, errors.New("leader changed"))
	a.ReevaluateSMS(t.Context())

	if !ue.SMSOverNAS() || sender.downlinkNasTransportCalls != 0 {
		t.Fatal("a grant that could not be re-evaluated changed")
	}

	handler.setAllowed(false, nil)
	a.ReevaluateSMS(t.Context())

	if ue.SMSOverNAS() || sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("SMS over NAS = %t, downlinks = %d, want the grant withdrawn and one SMS indication", ue.SMSOverNAS(), sender.downlinkNasTransportCalls)
	}

	a.SMSIndicationAcknowledged(t.Context(), ue, ue.Conn())
	handler.setAllowed(true, nil)
	a.ReevaluateSMS(t.Context())

	if ue.SMSOverNAS() || sender.downlinkNasTransportCalls != 2 {
		t.Fatalf("SMS over NAS = %t, downlinks = %d, want the grant left withdrawn and an \"available\" indication", ue.SMSOverNAS(), sender.downlinkNasTransportCalls)
	}

	a.ReevaluateSMS(t.Context())

	if sender.downlinkNasTransportCalls != 2 {
		t.Fatal("the \"available\" indication was sent twice")
	}

	if !a.GrantSMSOverNAS(t.Context(), ue, true) || !ue.SMSOverNAS() {
		t.Fatal("the grant was not restored by the registration")
	}
}

func TestARevocationDuringARegistrationWins(t *testing.T) {
	a, handler, _ := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)

	held, release := handler.holdDecision()
	granted := make(chan bool)

	go func() { granted <- a.GrantSMSOverNAS(t.Context(), ue, true) }()

	<-held
	handler.mu.Lock()
	handler.allowed, handler.hold = false, nil
	handler.mu.Unlock()

	reevaluated := make(chan struct{})

	go func() {
		a.ReevaluateSMS(t.Context())
		close(reevaluated)
	}()

	close(release)
	<-granted
	<-reevaluated

	if ue.SMSOverNAS() {
		t.Fatal("a registration that read the old configuration restored the revoked grant")
	}
}

func TestAnIdleUEGetsTheSMSIndicationWhenItReconnects(t *testing.T) {
	a, handler, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)

	handler.setAllowed(false, nil)
	a.ReevaluateSMS(t.Context())

	if ue.SMSOverNAS() || sender.downlinkNasTransportCalls != 0 {
		t.Fatal("the grant was not withdrawn, or an indication was sent to an idle UE")
	}

	connectSMSUE(t, a, ue, sender)
	a.SMSReachable(t.Context(), ue)

	if sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("downlinks = %d, want the pending SMS indication", sender.downlinkNasTransportCalls)
	}

	a.SMSReachable(t.Context(), ue)

	if sender.downlinkNasTransportCalls != 1 {
		t.Fatal("the SMS indication was sent twice")
	}
}

func TestSMSPagingWaitsOutAHandoverInProgress(t *testing.T) {
	a, _, _ := newSMSTestAMF(t)
	a.T3513Cfg = guard.TimerValue{Enable: true, ExpireTime: time.Hour, MaxRetryTimes: 1}
	ue := newIdleSMSUE(t, a)

	if err := ue.Procedures().Begin(procedure.N2Handover); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { _ = a.EnableUEReachabilityForSMS(ctx, "001019756139901") }()

	time.Sleep(100 * time.Millisecond)

	if ue.PagingState() != amf.PagingIdle {
		t.Fatal("paged during a handover")
	}

	ue.Procedures().End(procedure.N2Handover)

	waitFor(t, func() bool { return ue.PagingState() == amf.PagingAttempting })

	cancel()
	ue.StopPagingForTest(t.Context())
}

func TestTheSMSIndicationIsRetransmittedUntilT3555Expires(t *testing.T) {
	a, handler, sender := newSMSTestAMF(t)
	a.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: 10 * time.Millisecond, MaxRetryTimes: 2}
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)

	handler.setAllowed(false, nil)
	a.ReevaluateSMS(t.Context())

	deadline := time.Now().Add(time.Second)
	for ue.Conn().NASGuardActive() || ue.SMSIndicationPendingForTest() {
		if time.Now().After(deadline) {
			t.Fatal("T3555 never expired")
		}

		time.Sleep(5 * time.Millisecond)
	}

	if n := sender.downlinkNasTransportCalls; n != 3 {
		t.Fatalf("sent %d SMS indications, want the original and two retransmissions", n)
	}

	a.SMSReachable(t.Context(), ue)

	if n := sender.downlinkNasTransportCalls; n != 3 {
		t.Fatal("an SMS indication abandoned by T3555 was sent again")
	}
}

func TestTheSMSIndicationWaitsForAnotherProcedure(t *testing.T) {
	a, handler, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)

	ue.Conn().NASGuardForTest().Arm(time.Hour, 1, func(int32) {}, func() {})

	handler.setAllowed(false, nil)
	a.ReevaluateSMS(t.Context())

	if n := sender.downlinkNasTransportCalls; n != 0 {
		t.Fatalf("sent %d SMS indications while another procedure held the NAS guard", n)
	}

	ue.Conn().StopNASGuard(t.Context())
	a.SMSReachable(t.Context(), ue)

	if n := sender.downlinkNasTransportCalls; n != 1 {
		t.Fatalf("sent %d SMS indications once the guard was free, want 1", n)
	}
}

func TestAnUnacknowledgedSMSIndicationIsResentOnTheNextConnection(t *testing.T) {
	a, handler, sender := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	connectSMSUE(t, a, ue, sender)

	handler.setAllowed(false, nil)
	a.ReevaluateSMS(t.Context())

	if n := sender.downlinkNasTransportCalls; n != 1 {
		t.Fatalf("sent %d SMS indications, want 1", n)
	}

	connectSMSUE(t, a, ue, sender)
	a.SMSReachable(t.Context(), ue)

	if n := sender.downlinkNasTransportCalls; n != 2 {
		t.Fatalf("sent %d SMS indications, want it resent on the new connection", n)
	}

	a.SMSIndicationAcknowledged(t.Context(), ue, ue.Conn())
	connectSMSUE(t, a, ue, sender)
	a.SMSReachable(t.Context(), ue)

	if n := sender.downlinkNasTransportCalls; n != 2 {
		t.Fatal("an acknowledged SMS indication was sent again")
	}
}

func TestARevocationDuringARegistrationInProgressWins(t *testing.T) {
	a, handler, _ := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	held, release := handler.holdDecision()
	granted := make(chan bool)

	go func() { granted <- a.GrantSMSOverNAS(t.Context(), ue, true) }()

	<-held
	handler.mu.Lock()
	handler.allowed, handler.hold = false, nil
	handler.mu.Unlock()

	reevaluated := make(chan struct{})

	go func() {
		a.ReevaluateSMS(t.Context())
		close(reevaluated)
	}()

	close(release)
	<-granted
	<-reevaluated

	if ue.SMSOverNAS() {
		t.Fatal("a registration in progress that read the old configuration was granted SMS over NAS")
	}
}

func TestAnUnrelatedReevaluationDuringARegistrationKeepsTheGrant(t *testing.T) {
	a, handler, _ := newSMSTestAMF(t)
	ue := newIdleSMSUE(t, a)
	a.GrantSMSOverNAS(t.Context(), ue, false)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	held, release := handler.holdDecision()
	granted := make(chan bool)

	go func() { granted <- a.GrantSMSOverNAS(t.Context(), ue, true) }()

	<-held
	handler.mu.Lock()
	handler.hold = nil
	handler.mu.Unlock()

	reevaluated := make(chan struct{})

	go func() {
		a.ReevaluateSMS(t.Context())
		close(reevaluated)
	}()

	close(release)

	ok := <-granted

	<-reevaluated

	if !ok || !ue.SMSOverNAS() {
		t.Fatal("a re-evaluation that changed nothing withdrew SMS over NAS from a registration in progress")
	}
}
