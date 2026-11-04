// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func visitorAPN() *models.APNSubscription {
	return &models.APNSubscription{
		HPLMN:    visitorHPLMN,
		PDNTypes: []eps.PDNType{eps.PDNTypeIPv4v6},
		APNAMBR:  &models.Ambr{Uplink: models.MustParseBitRate("20 Mbps"), Downlink: models.MustParseBitRate("40 Mbps")},
		QCI:      9,
		MSISDN:   "15551234",
	}
}

func visitorInEPS(t *testing.T, home *fakeHPLMNClient) *smf.SMF {
	t.Helper()

	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	store.pcf = pcf
	s := smf.New(pcf, store, upf, amfCb, smf.WithSubscriptions(roamingSubscriptions{store}), smf.WithHPLMNClient(home))
	s.SetMME(mmeCb)

	establishEPSOnENBWith(t, s, visitorAPN())

	home.mu.Lock()
	opened := len(home.requests)
	home.mu.Unlock()

	if opened != 0 {
		t.Fatalf("home sessions opened for a PDN connection in EPS = %d, want none", opened)
	}

	return s
}

func TestVisitorSessionThatMovesFromEPSRegistersAtTheHomeNetworkInTheTransfer(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	s := visitorInEPS(t, home)

	ref, _, err := s.TransferIdleTo5GS(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, testTarget())
	if err != nil {
		t.Fatalf("TransferIdleTo5GS: %v", err)
	}

	h := home.session(t)
	waitRegistered(t, h)

	home.mu.Lock()
	req := home.requests[0]
	home.mu.Unlock()

	if req.PDUSessionID != movedPDUSessionID || req.Dnn != testDNN || req.Target != *testTarget() {
		t.Errorf("home session request = %+v, want the moved PDU session", req)
	}

	if err := s.ReleaseSmContext(context.Background(), ref, models.ReleaseFor(models.ReleaseDeregistration)); err != nil {
		t.Fatal(err)
	}

	if h.closes() != 1 {
		t.Errorf("home session closes = %d after the release, want 1", h.closes())
	}
}

func TestVisitorSessionMovingFromEPSWithoutAnHPLMNTargetIsRefused(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	s := visitorInEPS(t, home)

	_, _, err := s.TransferIdleTo5GS(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, nil)
	if !errors.Is(err, smf.ErrHPLMNTargetMissing) {
		t.Fatalf("TransferIdleTo5GS = %v, want %v", err, smf.ErrHPLMNTargetMissing)
	}
}

func TestVisitorSessionTheHomeNetworkRefusesOn5GSStaysInEPS(t *testing.T) {
	home := &fakeHPLMNClient{err: causeError{fgs.GSMCauseRequestedServiceOptionNotSubscribed}}
	s := visitorInEPS(t, home)

	if _, _, err := s.TransferIdleTo5GS(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, testTarget()); err == nil {
		t.Fatal("moved a session the home network refuses on 5GS")
	}

	if s.SessionCount() != 1 {
		t.Fatalf("sessions = %d, want the PDN connection kept in EPS", s.SessionCount())
	}
}

func TestAbandonedMoveOnto5GSUnbindsTheHPLMNSubscriptionAtOnce(t *testing.T) {
	for name, abandon := range map[string]func(*smf.SMF, string){
		"handover cancelled": func(s *smf.SMF, ref string) { _ = s.UpdateSmContextN2HandoverCanceled(context.Background(), ref) },
		"handover failed":    func(s *smf.SMF, ref string) { _ = s.UpdateSmContextN2HandoverFailed(context.Background(), ref, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			home := &fakeHPLMNClient{rules: hplmnRules()}
			s := visitorInEPS(t, home)

			ref, _, _, err := s.PrepareSmContextFromEPS(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, testTarget())
			if err != nil {
				t.Fatalf("PrepareSmContextFromEPS: %v", err)
			}

			h := home.session(t)
			if h.closes() != 0 {
				t.Fatalf("HPLMN subscription closes = %d before the move is abandoned, want 0", h.closes())
			}

			abandon(s, ref)

			if h.closes() != 1 {
				t.Fatalf("HPLMN subscription closes = %d after the move is abandoned, want 1", h.closes())
			}

			sc := s.GetSession(ref)
			if sc == nil || s.SessionCount() != 1 || accessOf(t, sc) != smf.Access4G {
				t.Fatalf("sessions = %d, want the PDN connection kept in EPS", s.SessionCount())
			}

			if _, _, _, err := s.PrepareSmContextFromEPS(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, testTarget()); err != nil {
				t.Fatalf("PrepareSmContextFromEPS after the abandoned move: %v", err)
			}

			home.mu.Lock()
			opened := len(home.sessions)
			home.mu.Unlock()

			if opened != 2 {
				t.Fatalf("HPLMN subscriptions opened = %d, want a fresh one for the next move", opened)
			}

			if err := s.ReleaseSmContext(context.Background(), ref, models.ReleaseFor(models.ReleaseDeregistration)); err != nil {
				t.Fatal(err)
			}

			if h.closes() != 1 {
				t.Errorf("abandoned HPLMN subscription closes = %d after the release, want 1", h.closes())
			}
		})
	}
}

func TestVisitorPDNConnectionNeedsTheHomeAPNSubscription(t *testing.T) {
	pcf, store, upf, amfCb, _ := interworkingFakes()
	store.pcf = pcf
	s := smf.New(pcf, store, upf, amfCb, smf.WithSubscriptions(roamingSubscriptions{store}))

	req := epsRequest(3)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	if _, err := s.CreateEPSSession(context.Background(), req); !errors.Is(err, smf.ErrHPLMNTargetMissing) {
		t.Fatalf("CreateEPSSession without the home APN subscription = %v, want %v", err, smf.ErrHPLMNTargetMissing)
	}
}

func TestVisitorPDNConnectionAppliesTheHomeAPNSubscription(t *testing.T) {
	pcf, store, upf, amfCb, _ := interworkingFakes()
	store.pcf = pcf
	s := smf.New(pcf, store, upf, amfCb, smf.WithSubscriptions(roamingSubscriptions{store}))

	req := epsRequest(3)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai
	req.Subscription = visitorAPN()
	req.Subscription.PDNTypes = []eps.PDNType{eps.PDNTypeIPv4}

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if bearer.PDNType != eps.PDNTypeIPv4 || bearer.ESMCause != eps.ESMCausePDNTypeIPv4OnlyAllowed {
		t.Fatalf("PDN type = %v, cause %v; want IPv4 only, as the home APN subscribes", bearer.PDNType, bearer.ESMCause)
	}

	single := epsRequest(3)
	single.APN = testDNN
	single.PDUSessionID = movedPDUSessionID + 1
	single.EPSBearerIdentity = epsTestEBI + 1
	single.Snssai = testSnssai
	single.Subscription = visitorAPN()
	single.Subscription.PDNTypes = []eps.PDNType{eps.PDNTypeIPv4, eps.PDNTypeIPv6}

	bearer2, err := s.CreateEPSSession(context.Background(), single)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if bearer2.PDNType != eps.PDNTypeIPv4 || bearer2.ESMCause != eps.ESMCauseSingleAddressBearersOnly {
		t.Fatalf("PDN type = %v, cause %v; want IPv4 with #52, as the home APN subscribes IPv4_OR_IPv6", bearer2.PDNType, bearer2.ESMCause)
	}

	if bearer.HPLMNSnssai != nil {
		t.Errorf("home S-NSSAI = %+v, want none: under LBO the PCO names the VPLMN S-NSSAI (TS 23.501 §5.15.7.1)", bearer.HPLMNSnssai)
	}

	pcf.mu.Lock()

	var got smf.SubscribedQoS
	for _, c := range pcf.associations {
		got = c.Subscribed
	}
	pcf.mu.Unlock()

	if got.SessionAMBR.Uplink.Bps() != 20_000_000 || got.SessionAMBR.Downlink.Bps() != 40_000_000 {
		t.Fatalf("subscribed Session-AMBR = %+v, want the home APN-AMBR 20/40 Mbps", got.SessionAMBR)
	}
}

func TestWithdrawnHomeAPNReleasesThePDNConnection(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	store.pcf = pcf
	s := smf.New(pcf, store, upf, amfCb, smf.WithSubscriptions(roamingSubscriptions{store}), smf.WithHPLMNClient(home))
	s.SetMME(mmeCb)

	sc := establishEPSOnENBWith(t, s, visitorAPN())

	s.UpdateAPNSubscription(context.Background(), sc.Ref, nil)

	if err := s.ReconcileSession(context.Background(), sc.Ref); err != nil {
		t.Fatal(err)
	}

	mmeCb.mu.Lock()
	reactivations := len(mmeCb.reactivated)
	mmeCb.mu.Unlock()

	if reactivations == 0 && s.GetSession(sc.Ref) != nil {
		t.Fatal("a PDN connection whose APN the home network withdrew was kept")
	}
}
