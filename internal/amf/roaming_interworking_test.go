// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehome"
	"github.com/ellanetworks/core/internal/roaming/wire"
)

type n26Policy bool

func (p n26Policy) Allowed(context.Context, string) (bool, error) { return bool(p), nil }

func (f *roamingFixture) registeredVisitor(t *testing.T) (*UeContext, *UeConn) {
	t.Helper()

	f.amf.N26 = n26Policy(true)

	ue, conn := f.visitorUE(t)
	f.authenticate(t, ue, conn)
	f.registerAtHome(t, ue, conn)

	if err := f.amf.RegisterUE(t.Context(), ue); err != nil {
		t.Fatal(err)
	}

	ue.ForceStateForTest(Registered)

	return ue, conn
}

func TestVisitorMovingToEPSPassesItsHomeRegistrationWithoutAPurge(t *testing.T) {
	f := newRoamingFixture(t)
	ue, _ := f.registeredVisitor(t)
	supi := "imsi-" + roamerIMSI

	if len(f.home.Subscriptions()) != 1 {
		t.Fatalf("SDM subscriptions = %d before the move, want 1", len(f.home.Subscriptions()))
	}

	if err := f.amf.EPSContextAck(t.Context(), ue.Supi(), nil); err != nil {
		t.Fatalf("EPSContextAck: %v", err)
	}

	f.amf.SettleHomeNetwork()

	v, ok := ue.Visitor()
	if !ok || v.Registered || v.SDM != nil || v.ConfirmURI != "" {
		t.Fatalf("visitor = %+v, want its home registration handed to EPS", v)
	}

	if len(f.home.Subscriptions()) != 0 {
		t.Error("the SDM subscription of the visitor that moved to EPS was kept")
	}

	if err := f.amf.HomeDeregistration(t.Context(), v.PartnerID, roamerIMSI, wire.DeregistrationData{DeregReason: wire.Dereg5GSToEPSMobility, AccessType: wire.Access3GPP}); err != nil {
		t.Fatalf("the home UDM's 5GS_TO_EPS_MOBILITY notification for the moved visitor = %v, want 204", err)
	}

	if !f.amf.HoldsUE(roamerIMSI) {
		t.Error("the notification removed the context kept for a return from EPS")
	}

	f.amf.DeregisterAndRemoveUeContext(t.Context(), ue, models.ReleaseFor(models.ReleaseDeregistration))
	f.amf.SettleHomeNetwork()

	if f.home.Purged(supi) {
		t.Error("the AMF purged a registration that passed to the MME")
	}
}

func TestFiveGSToEPSMobilityNotificationOfAVisitorRegisteredHere(t *testing.T) {
	f := newRoamingFixture(t)
	ue, _ := f.registeredVisitor(t)
	v, _ := ue.Visitor()

	if err := f.amf.HomeDeregistration(t.Context(), "other-partner", roamerIMSI, wire.DeregistrationData{DeregReason: wire.Dereg5GSToEPSMobility}); !errors.Is(err, roaming.ErrNoVisitorContext) {
		t.Fatalf("notification from another partner = %v, want ErrNoVisitorContext", err)
	}

	if err := f.amf.HomeDeregistration(t.Context(), v.PartnerID, roamerIMSI, wire.DeregistrationData{DeregReason: wire.Dereg5GSToEPSMobility, AccessType: wire.Access3GPP}); err != nil {
		t.Fatalf("HomeDeregistration: %v", err)
	}

	f.amf.SettleHomeNetwork()

	if got, _ := ue.Visitor(); got.Registered {
		t.Fatal("a visitor that registered in EPS outside this node is still registered here")
	}

	if f.amf.HoldsUE(roamerIMSI) {
		f.amf.DeregisterAndRemoveUeContext(t.Context(), ue, models.ReleaseFor(models.ReleaseDeregistration))
		f.amf.SettleHomeNetwork()
	}

	if f.home.Purged("imsi-" + roamerIMSI) {
		t.Error("the AMF purged a registration the home network had cancelled")
	}

	if err := f.amf.HomeDeregistration(t.Context(), v.PartnerID, roamerIMSI, wire.DeregistrationData{DeregReason: wire.Dereg5GSToEPSMobility}); !errors.Is(err, roaming.ErrNoVisitorContext) {
		t.Fatalf("a second notification = %v, want ErrNoVisitorContext", err)
	}
}

func TestEPSInterworkingOfAVisitorFollowsTheHomeSubscription(t *testing.T) {
	f := newRoamingFixture(t)
	f.home.Update(roamerIMSI, func(s *fakehome.Subscriber) {
		s.SMFSel = &wire.SmfSelectionSubscriptionData{SubscribedSnssaiInfos: map[string]wire.SnssaiInfo{
			"1": {DnnInfos: []wire.DnnInfo{
				{Dnn: "internet", DefaultDnnIndicator: true, LboRoamingAllowed: true, IwkEpsInd: true},
				{Dnn: "iot", LboRoamingAllowed: true},
			}},
		}}
	})

	ue, _ := f.registeredVisitor(t)

	profile, err := f.amf.SubscriberProfile(t.Context(), ue.Supi())
	if err != nil || !profile.EPSInterworking {
		t.Fatalf("profile = %+v, %v, want EPS interworking for a visitor the policy allows", profile, err)
	}

	if !ue.EPSInterworkingSubscribed(models.Snssai{Sst: 1}, "internet") {
		t.Error("a DNN with iwkEpsInd gets no EPS bearer identity")
	}

	if ue.EPSInterworkingSubscribed(models.Snssai{Sst: 1}, "iot") {
		t.Error("a DNN without iwkEpsInd gets an EPS bearer identity")
	}

	ue.updateVisitor(func(v *roaming.Visitor5G) {
		am := *v.Subscription.AmData
		am.CoreNetworkTypeRestrictions = []wire.CoreNetworkType{wire.CoreNetworkEPC}
		data := *v.Subscription
		data.AmData = &am
		v.Subscription = &data
	})

	if profile, err := f.amf.SubscriberProfile(t.Context(), ue.Supi()); err != nil || profile.EPSInterworking {
		t.Fatalf("profile = %+v, %v, want no EPS interworking when the home restricts EPC", profile, err)
	}

	f.amf.N26 = n26Policy(false)

	ue.updateVisitor(func(v *roaming.Visitor5G) { v.Subscription.AmData.CoreNetworkTypeRestrictions = nil })

	if profile, err := f.amf.SubscriberProfile(t.Context(), ue.Supi()); err != nil || profile.EPSInterworking {
		t.Fatalf("profile = %+v, %v, want no EPS interworking when the policy refuses N26", profile, err)
	}
}

func TestVisitorArrivingFromEPSRegistersAtItsHomeUDMAsAMobilityRegistration(t *testing.T) {
	f := newRoamingFixture(t)
	f.amf.N26 = n26Policy(true)

	partner := "partner-1"
	f.db.visitors[roamerIMSI] = &db.Subscriber{Imsi: roamerIMSI, ProfileID: "roamers", RoamingPartnerID: &partner}

	supi, err := etsi.NewSUPIFromIMSI(roamerIMSI)
	if err != nil {
		t.Fatal(err)
	}

	ue := NewUeContext()
	ue.SetSupi(supi)
	conn := connectTestUE(t, f.amf, ue)

	if err := f.amf.AdmitVisitor(t.Context(), ue); err != nil || !ue.IsVisitor() {
		t.Fatalf("AdmitVisitor = %v, visitor = %v", err, ue.IsVisitor())
	}

	transferred := models.Ambr{Uplink: models.MustParseBitRate("10 Mbps"), Downlink: models.MustParseBitRate("20 Mbps")}

	profile, err := f.amf.ArrivingVisitorProfile(t.Context(), supi, transferred)
	if err != nil {
		t.Fatalf("ArrivingVisitorProfile: %v", err)
	}

	if *profile.Ambr != transferred || !profile.EPSInterworking || len(profile.AllowedNssai) != 1 {
		t.Fatalf("arriving profile = %+v (AMBR %+v), want the local slices and the UE-AMBR the MME enforced", profile, *profile.Ambr)
	}

	done := make(chan error, 1)

	f.amf.RequestHomeRegistration(t.Context(), ue, conn, false, func(_ context.Context, err error) { done <- err })

	if err := waitDone(t, done); err != nil {
		t.Fatalf("RequestHomeRegistration: %v", err)
	}

	reg, ok := f.home.Registration("imsi-" + roamerIMSI)
	if !ok {
		t.Fatal("no UECM registration at the home UDM")
	}

	if reg.InitialRegistrationInd != nil || reg.RatType != wire.RatNR {
		t.Errorf("UECM registration %+v, want an NR mobility registration without initialRegistrationInd", reg)
	}

	if len(f.home.Subscriptions()) != 1 {
		t.Errorf("SDM subscriptions = %d, want 1", len(f.home.Subscriptions()))
	}

	if v, _ := ue.Visitor(); !v.Registered || v.Subscription == nil {
		t.Errorf("visitor = %+v, want registered with its home subscription", v)
	}
}
