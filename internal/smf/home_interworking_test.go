// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
)

func visitorMovedTo5GS(t *testing.T, home *fakeHomeNetwork) (*smf.SMF, *fakeAMF, string) {
	t.Helper()

	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	store.pcf = pcf
	s := smf.New(pcf, store, upf, amfCb, smf.WithSubscriptions(roamingSubscriptions{store}), smf.WithHomeNetwork(home))
	s.SetMME(mmeCb)

	establishEPSOnENB(t, s)

	home.mu.Lock()
	opened := len(home.requests)
	home.mu.Unlock()

	if opened != 0 {
		t.Fatalf("home sessions opened for a PDN connection in EPS = %d, want none", opened)
	}

	ref, err := s.TransferIdle(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access5G)
	if err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	return s, amfCb, ref
}

func TestVisitorSessionThatMovedFromEPSRegistersAtTheHomeNetwork(t *testing.T) {
	home := &fakeHomeNetwork{rules: homeRules()}
	s, _, ref := visitorMovedTo5GS(t, home)

	s.BindHomeSessions(context.Background(), testSUPI())

	h := home.session(t)
	waitRegistered(t, h)

	home.mu.Lock()
	req := home.requests[0]
	home.mu.Unlock()

	if req.PDUSessionID != movedPDUSessionID || req.Dnn != testDNN || req.Snssai != *testSnssai {
		t.Errorf("home session request = %+v, want the moved PDU session", req)
	}

	s.BindHomeSessions(context.Background(), testSUPI())

	home.mu.Lock()
	opened := len(home.requests)
	home.mu.Unlock()

	if opened != 1 {
		t.Errorf("home sessions opened = %d after a second bind, want the bound session left alone", opened)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("binding the session removed it")
	}

	if err := s.ReleaseSmContext(context.Background(), ref, models.ReleaseFor(models.ReleaseDeregistration)); err != nil {
		t.Fatal(err)
	}

	if h.closes() != 1 {
		t.Errorf("home session closes = %d after the release, want 1", h.closes())
	}
}

func TestVisitorSessionTheHomeNetworkRefusesOn5GSIsReleased(t *testing.T) {
	home := &fakeHomeNetwork{err: causeError{fgs.GSMCauseRequestedServiceOptionNotSubscribed}}
	s, amfCb, _ := visitorMovedTo5GS(t, home)

	s.BindHomeSessions(context.Background(), testSUPI())

	amfCb.mu.Lock()
	calls := len(amfCb.releaseCalls)

	var (
		cmd *fgs.PDUSessionReleaseCommand
		err error
	)

	if calls > 0 {
		cmd, err = fgs.ParsePDUSessionReleaseCommand(amfCb.releaseCalls[0].n1Msg)
	}
	amfCb.mu.Unlock()

	if err != nil || cmd == nil || cmd.Cause != fgs.GSMCauseRequestedServiceOptionNotSubscribed {
		t.Fatalf("release command = %+v, %v, want cause #33 from the home network's refusal", cmd, err)
	}
}

func TestVisitorPDNConnectionNamesTheHomeSlice(t *testing.T) {
	pcf, store, upf, amfCb, _ := interworkingFakes()
	store.pcf = pcf
	s := smf.New(pcf, store, upf, amfCb, smf.WithSubscriptions(roamingSubscriptions{store}))

	req := epsRequest(3)
	req.APN = testDNN
	req.PDUSessionID = movedPDUSessionID
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if bearer.HomeSnssai == nil || bearer.HomeSnssai.Sst != testSnssai.Sst || bearer.HomeSnssai.Sd != models.NormalizeSD(testSnssai.Sd) {
		t.Errorf("home S-NSSAI = %+v, want the HPLMN value of %+v for the UE's mapped NSSAI on 5GS", bearer.HomeSnssai, testSnssai)
	}
}

func TestVisitorSessionStaysUnboundWhileTheHomeNetworkIsUnreachable(t *testing.T) {
	home := &fakeHomeNetwork{err: causeError{fgs.GSMCauseNetworkFailure}}
	s, amfCb, ref := visitorMovedTo5GS(t, home)

	s.BindHomeSessions(context.Background(), testSUPI())

	amfCb.mu.Lock()
	calls := len(amfCb.releaseCalls)
	amfCb.mu.Unlock()

	if calls != 0 || s.GetSession(ref) == nil {
		t.Fatal("an unreachable home network released the session")
	}

	home.mu.Lock()
	home.err = nil
	home.rules = homeRules()
	home.mu.Unlock()

	s.BindHomeSessions(context.Background(), testSUPI())

	home.mu.Lock()
	opened := len(home.sessions)
	home.mu.Unlock()

	if opened != 1 {
		t.Fatalf("home sessions opened after the home network came back = %d, want 1", opened)
	}
}
