// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smsf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type fakeSMSHandler struct {
	mu      sync.Mutex
	pending bool
	denied  bool
	events  []string
	hold    chan struct{}
	held    chan struct{}
}

func (h *fakeSMSHandler) Uplink(context.Context, string, []byte) {}

func (h *fakeSMSHandler) UEReachable(_ context.Context, imsi string) {
	h.record("reachable " + imsi)
}

func (h *fakeSMSHandler) AllowedEach(_ context.Context, imsis []string) (map[string]bool, error) {
	h.mu.Lock()
	ok, hold, held := !h.denied, h.hold, h.held
	h.hold, h.held = nil, nil
	h.mu.Unlock()

	if hold != nil {
		close(held)
		<-hold
	}

	allowed := make(map[string]bool, len(imsis))
	for _, imsi := range imsis {
		allowed[imsi] = ok
	}

	return allowed, nil
}

func (h *fakeSMSHandler) TransactionPending(string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.pending
}

func (h *fakeSMSHandler) record(e string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.events = append(h.events, e)
}

func (h *fakeSMSHandler) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return slices.Clone(h.events)
}

func smsTestMME(t *testing.T) (*MME, *fakeSMSHandler, *captureConn) {
	t.Helper()

	m := newTestMME(t)
	handler := &fakeSMSHandler{}
	m.SMS = handler

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	radio := &captureConn{}
	m.IndexRadioForTest(radio, []SupportedTAI{{Tai: models.Tai{PlmnID: &plmn, Tac: "000001"}}})

	return m, handler, radio
}

func grantSMS(t *testing.T, m *MME, ue *UeContext) {
	t.Helper()

	if m.DecideSMS(t.Context(), ue, true) != SMSGranted {
		t.Fatal("precondition: SMS must be granted")
	}
}

func TestSMSIsSentOverTheUEsConnection(t *testing.T) {
	m, _, _ := smsTestMME(t)
	ue, cc := securedUE(t, m)
	grantSMS(t, m, ue)

	cpData := []byte{0x09, 0x01, 0x02}

	if err := m.EnableUEReachabilityForSMS(t.Context(), ue.imsiOrEmpty()); err != nil {
		t.Fatalf("EnableUEReachabilityForSMS: %v", err)
	}

	if err := m.SendSMS(t.Context(), ue.imsiOrEmpty(), cpData); err != nil {
		t.Fatalf("SendSMS: %v", err)
	}

	if cc.count() != 1 {
		t.Fatalf("sent = %d, want 1", cc.count())
	}

	wire := decodeDownlinkNAS(t, cc.snapshot()[0])

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect: %v", err)
	}

	msg, err := eps.ParseMessage(plain, nas.DirectionDownlink)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dl, ok := msg.(*eps.DownlinkNASTransport)
	if !ok || !bytes.Equal(dl.NASMessageContainer, cpData) {
		t.Fatalf("downlink = %#v, want a DOWNLINK NAS TRANSPORT carrying %x", msg, cpData)
	}
}

func TestSMSIsRefusedForAUEThatCannotReceiveIt(t *testing.T) {
	m, _, _ := smsTestMME(t)

	if err := m.EnableUEReachabilityForSMS(t.Context(), "001010000000999"); !errors.Is(err, smsf.ErrNotRegisteredForSMS) {
		t.Fatalf("unknown UE: err = %v, want %v", err, smsf.ErrNotRegisteredForSMS)
	}

	ue, _ := securedUE(t, m)

	if err := m.SendSMS(t.Context(), ue.imsiOrEmpty(), []byte{0x09}); !errors.Is(err, smsf.ErrNotRegisteredForSMS) {
		t.Fatalf("UE without SMS: err = %v, want %v", err, smsf.ErrNotRegisteredForSMS)
	}
}

func TestSMSIsNotSentToAnIdleUEWithoutPagingIt(t *testing.T) {
	m, _, radio := smsTestMME(t)
	ue := idleRegisteredUE(t, m)
	grantSMS(t, m, ue)

	if err := m.SendSMS(t.Context(), ue.imsiOrEmpty(), []byte{0x09}); !errors.Is(err, smsf.ErrUnreachable) {
		t.Fatalf("err = %v, want %v", err, smsf.ErrUnreachable)
	}

	if radio.count() != 0 {
		t.Fatalf("pages = %d, want 0", radio.count())
	}
}

func TestAnIdleUEIsPagedAndReachableForSMSOnceTheProcedureSettles(t *testing.T) {
	m, _, radio := smsTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour
	ue := idleRegisteredUE(t, m)
	grantSMS(t, m, ue)

	done := make(chan error, 1)

	go func() { done <- m.EnableUEReachabilityForSMS(context.Background(), ue.imsiOrEmpty()) }()

	deadline := time.Now().Add(time.Second)
	for ue.PagingState() != PagingAttempting {
		if time.Now().After(deadline) {
			t.Fatal("the idle UE was not paged")
		}

		time.Sleep(time.Millisecond)
	}

	establishResumeForTest(m, ue, &captureConn{}, 9)

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

	if radio.count() != 1 {
		t.Fatalf("pages = %d, want 1", radio.count())
	}
}

func TestSMSPagingIsSentOnceAndBoundedByTheCaller(t *testing.T) {
	m, _, radio := smsTestMME(t)
	m.pagingCfg.ExpireTime = 5 * time.Millisecond
	m.pagingCfg.MaxRetryTimes = 3
	ue := idleRegisteredUE(t, m)
	grantSMS(t, m, ue)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	if err := m.EnableUEReachabilityForSMS(ctx, ue.imsiOrEmpty()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want %v", err, context.DeadlineExceeded)
	}

	if n := radio.count(); n != 1 {
		t.Fatalf("pages = %d, want one without T3413 retransmission", n)
	}

	if m.pagingActive(ue) || ue.PagingState() != PagingIdle {
		t.Fatal("the SMS page was left pending after the caller gave up")
	}
}

func TestRemovingTheUEContextDeregistersItFromSMS(t *testing.T) {
	m, _, _ := smsTestMME(t)
	ue, _ := securedUE(t, m)
	grantSMS(t, m, ue)
	imsi := ue.imsiOrEmpty()

	m.RemoveUe(ue)

	if granted, _ := m.SMSRoute(imsi); granted {
		t.Fatal("the MME still routes SMS to a removed UE")
	}
}

func TestCompletingARegistrationReportsTheUEReachableForSMS(t *testing.T) {
	m, handler, _ := smsTestMME(t)
	ue, _ := securedUE(t, m)
	imsi := ue.imsiOrEmpty()

	grantSMS(t, m, ue)
	m.SMSReachable(t.Context(), ue)
	m.DecideSMS(t.Context(), ue, false)
	m.SMSReachable(t.Context(), ue)

	if got := handler.snapshot(); !slices.Equal(got, []string{"reachable " + imsi}) {
		t.Fatalf("events = %v", got)
	}
}

func TestARevocationDuringARegistrationWins(t *testing.T) {
	m, handler, _ := smsTestMME(t)
	ue := idleRegisteredUE(t, m)
	grantSMS(t, m, ue)

	held, release := make(chan struct{}), make(chan struct{})

	handler.mu.Lock()
	handler.hold, handler.held = release, held
	handler.mu.Unlock()

	decision := make(chan SMSDecision)

	go func() { decision <- m.DecideSMS(t.Context(), ue, true) }()

	<-held

	handler.mu.Lock()
	handler.denied = true
	handler.mu.Unlock()

	reevaluated := make(chan struct{})

	go func() {
		m.ReevaluateSMS(t.Context())
		close(reevaluated)
	}()

	close(release)
	<-decision
	<-reevaluated

	if ue.SMSOnly() {
		t.Fatal("a registration that read the old configuration restored the revoked grant")
	}
}

func TestARevocationClearsTheGrantWithoutSignallingTheUE(t *testing.T) {
	m, handler, _ := smsTestMME(t)
	ue, cc := securedUE(t, m)
	grantSMS(t, m, ue)

	handler.mu.Lock()
	handler.denied = true
	handler.mu.Unlock()

	m.ReevaluateSMS(t.Context())

	if ue.SMSOnly() {
		t.Fatal("the grant survived the revocation")
	}

	if n := cc.count(); n != 0 {
		t.Fatalf("sent %d NAS messages, want none: the UE learns of the revocation at its next TAU (TS 23.272 §C.8.2.1)", n)
	}
}

func TestAnOpenSMSTransactionHoldsTheS1Connection(t *testing.T) {
	m, handler, _ := smsTestMME(t)
	ue, _ := securedUE(t, m)

	if ue.Conn().MTSignallingPending() {
		t.Fatal("signalling pending with no SMS transaction")
	}

	handler.mu.Lock()
	handler.pending = true
	handler.mu.Unlock()

	if !ue.Conn().MTSignallingPending() {
		t.Fatal("an open SMS transaction does not hold the S1 connection")
	}
}

func TestSMSToAUECompletingItsAttachWaitsForTheAttachComplete(t *testing.T) {
	m, _, _ := smsTestMME(t)
	ue, _ := securedUE(t, m)
	grantSMS(t, m, ue)
	ue.ForceStateForTest(EMMRegistrationInitiated)

	if granted, connected := m.SMSRoute(ue.imsiOrEmpty()); !granted || !connected {
		t.Fatalf("route = granted %t, connected %t, want the MME to hold a UE completing its attach", granted, connected)
	}

	done := make(chan error, 1)

	go func() { done <- m.EnableUEReachabilityForSMS(t.Context(), ue.imsiOrEmpty()) }()

	select {
	case err := <-done:
		t.Fatalf("reachability reported (%v) before the Attach Complete", err)
	case <-time.After(50 * time.Millisecond):
	}

	ue.ForceStateForTest(EMMRegistered)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("EnableUEReachabilityForSMS: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reachability not reported after the Attach Complete")
	}
}

func TestAnUnrelatedReevaluationDuringAnAttachKeepsTheGrant(t *testing.T) {
	m, handler, _ := smsTestMME(t)
	ue := idleRegisteredUE(t, m)

	held, release := make(chan struct{}), make(chan struct{})

	handler.mu.Lock()
	handler.hold, handler.held = release, held
	handler.mu.Unlock()

	decision := make(chan SMSDecision)

	go func() { decision <- m.DecideSMS(t.Context(), ue, true) }()

	<-held

	reevaluated := make(chan struct{})

	go func() {
		m.ReevaluateSMS(t.Context())
		close(reevaluated)
	}()

	close(release)

	d := <-decision

	<-reevaluated

	if d != SMSGranted || !ue.SMSOnly() {
		t.Fatalf("decision = %d, want the grant kept after a re-evaluation that changed nothing", d)
	}
}
