// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

const interworkingVisitorIMSI = "999700000000001"

type n26Policy bool

func (p n26Policy) Allowed(context.Context, string) (bool, error) { return bool(p), nil }

type interworkingPartners struct {
	partners []db.RoamingPartner
}

func (p interworkingPartners) ListRoamingPartners(context.Context) ([]db.RoamingPartner, error) {
	return p.partners, nil
}

func (interworkingPartners) CountSubscribersAndVisitors(context.Context) (int, error) { return 1, nil }

func (interworkingPartners) SetVisitorMSISDN(context.Context, string, string) error { return nil }

func interworkingVisitorMME(t *testing.T, allowed bool) *MME {
	t.Helper()

	partner, peer := "partner", "dea"
	store := lookupBearerStore{sub: &db.Subscriber{Imsi: interworkingVisitorIMSI, ProfileID: "test-profile", RoamingPartnerID: &partner}}

	m := New(udm.New(newFakeCredStore(), noopKeyResolver), store, &fakeSessionManager{})
	m.Roaming = interworkingPartners{partners: []db.RoamingPartner{{
		ID: partner, Name: "partner", ProfileID: "test-profile", DiameterPeerID: &peer,
		PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}},
	}}}
	m.N26 = n26Policy(allowed)

	return m
}

func TestForwardRelocationAdmitsAVisitorOfAPartnerWithBothAgreements(t *testing.T) {
	m := interworkingVisitorMME(t, true)
	reg := newFakeRegistrar()
	m.Registrations = reg
	m.FiveGS = &fakeFiveGSPeer{}
	target := newRelocationTarget(t, m)

	req := relocationRequest()
	req.SUPI = mustSUPI(interworkingVisitorIMSI)

	done := make(chan error, 1)

	go func() {
		_, err := m.ForwardRelocation(context.Background(), req)
		done <- err
	}()

	hoReq := target.awaitHandoverRequest(t)
	target.admit(t, hoReq)

	if err := <-done; err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	ue, ok := m.LookupUe(hoReq.MMEUES1APID)
	if !ok {
		t.Fatal("the relocated UE context is gone")
	}

	v, ok := ue.Visitor()
	if !ok || v.PartnerID != "partner" || v.Registered || v.Subscription != nil {
		t.Fatalf("visitor = %+v, %v, want an admitted visitor not yet registered at its home HSS", v, ok)
	}

	if !ue.FiveGSInterworkingAllowed() {
		t.Error("the arrived visitor may not move back to 5GS")
	}

	if _, ok := m.MarkHandoverCommitting(ue, target.conn, 42); !ok {
		t.Fatal("the Handover Notify did not match the prepared handover")
	}

	if _, _, _, _, ok := m.FinishHandoverCommit(ue, target.conn, 42); !ok {
		t.Fatal("the handover did not commit")
	}

	m.CompleteRelocation(context.Background(), ue)

	if len(reg.registered) != 1 || reg.registered[0] != interworkingVisitorIMSI {
		t.Errorf("registrations = %v, want the visitor registered on the MME", reg.registered)
	}

	if !m.HPLMNRegistrationDue(ue) || !ue.arrivedByHandoverFrom5GS.Load() {
		t.Error("the visitor that arrived by handover is not due to register at its home HSS at its TAU")
	}
}

func TestForwardRelocationRefusesAVisitorWithoutN26(t *testing.T) {
	m := interworkingVisitorMME(t, false)
	target := newRelocationTarget(t, m)

	req := relocationRequest()
	req.SUPI = mustSUPI(interworkingVisitorIMSI)

	_, err := m.ForwardRelocation(context.Background(), req)
	if !errors.Is(err, interworking.ErrN26NotAllowed) {
		t.Fatalf("error = %v, want ErrN26NotAllowed", err)
	}

	var refusal interworking.TargetRefusal
	if !errors.As(err, &refusal) || refusal.Cause.Value != s1ap.CauseRadioNetworkHOTargetNotAllowed {
		t.Fatalf("error = %v, want a handover-target-not-allowed refusal", err)
	}

	if len(target.handoverRequests(t)) != 0 {
		t.Error("the target eNB was asked to admit the UE")
	}
}

func TestIdleMoveOfAVisitorTo5GSLeavesNoHomeRegistrationToPurge(t *testing.T) {
	m := interworkingVisitorMME(t, true)
	ue, guti := idleMobilityUE(t, m)
	ue.SetVisitorForTest(roaming.Visitor{PartnerID: "partner", Registered: true, Confirmed: true, Subscription: &s6a.SubscriptionData{}})
	ue.SetAccess(Access{Allow4G: true, FiveGSInterworking: true})

	resp, err := m.MMContext(t.Context(), interworking.MMContextRequest{
		MappedEPSGUTI: guti,
		EPSNAS:        enclosedTAU(t, ue, nas.MakeCount(0, 0)),
	})
	if err != nil {
		t.Fatalf("MMContext: %v", err)
	}

	if len(resp.PDNConnections) != 1 {
		t.Fatalf("PDN connections handed out = %d, want 1", len(resp.PDNConnections))
	}

	if v, _ := ue.Visitor(); !v.Registered {
		t.Fatal("handing out the context dropped the home registration before the move completed")
	}

	if err := m.MMContextAck(t.Context(), resp.SUPI, []uint8{3}); err != nil {
		t.Fatalf("MMContextAck: %v", err)
	}

	if v, _ := ue.Visitor(); v.Registered {
		t.Error("the visitor that moved to 5GS still holds a home registration, so its removal would purge it")
	}
}

func TestMMContextRefusesAVisitorWithoutN26(t *testing.T) {
	m := interworkingVisitorMME(t, false)
	ue, guti := idleMobilityUE(t, m)
	ue.SetAccess(Access{Allow4G: true, FiveGSInterworking: true})

	_, err := m.MMContext(t.Context(), interworking.MMContextRequest{
		MappedEPSGUTI: guti,
		EPSNAS:        enclosedTAU(t, ue, nas.MakeCount(0, 0)),
	})
	if !errors.Is(err, interworking.ErrN26NotAllowed) {
		t.Fatalf("error = %v, want ErrN26NotAllowed", err)
	}
}

func TestResolveAccessHonoursTheN26PolicyAndTheHomeCoreNetworkRestriction(t *testing.T) {
	m := interworkingVisitorMME(t, true)
	ue := idleRegisteredUE(t, m)

	access, err := ResolveAccess(t.Context(), m, ue.IMSI())
	if err != nil || !access.FiveGSInterworking {
		t.Fatalf("access = %+v, %v, want 5GS interworking for a visitor the policy allows", access, err)
	}

	notAllowed := s6a.CNR5GCNotAllowed
	ue.SetVisitorForTest(roaming.Visitor{PartnerID: "partner", Subscription: &s6a.SubscriptionData{CoreNetworkRestrictions: &notAllowed}})

	access, err = ResolveAccess(t.Context(), m, ue.IMSI())
	if err != nil || access.FiveGSInterworking {
		t.Fatalf("access = %+v, %v, want no 5GS interworking when the home subscription disallows 5GC", access, err)
	}

	m.N26 = n26Policy(false)

	ue.SetVisitorForTest(roaming.Visitor{PartnerID: "partner", Subscription: &s6a.SubscriptionData{}})

	access, err = ResolveAccess(t.Context(), m, ue.IMSI())
	if err != nil || access.FiveGSInterworking || !access.Allow4G {
		t.Fatalf("access = %+v, %v, want 4G only when the policy refuses N26", access, err)
	}
}
