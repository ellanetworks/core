// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/fakehss"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/udm"
)

type n26Store struct {
	sub     *db.Subscriber
	subErr  error
	partner *db.RoamingPartner
	profile *db.Profile
}

func (s n26Store) GetSubscriberOrVisitor(context.Context, string) (*db.Subscriber, error) {
	return s.sub, s.subErr
}

func (s n26Store) GetRoamingPartner(context.Context, string) (*db.RoamingPartner, error) {
	if s.partner == nil {
		return nil, db.ErrNotFound
	}

	return s.partner, nil
}

func (s n26Store) GetProfileByID(context.Context, string) (*db.Profile, error) {
	return s.profile, nil
}

func n26Partner() *db.RoamingPartner {
	sepp, peer := "sepp.5gc.mnc070.mcc999.3gppnetwork.org", "dea"

	return &db.RoamingPartner{
		ID:             "p",
		Name:           "partner",
		ProfileID:      "partner-profile",
		SEPPFQDN:       &sepp,
		DiameterPeerID: &peer,
		PLMNs:          []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}},
	}
}

func TestN26IsOfferedToHomeSubscribersAndToVisitorsOfPartnersWithBothAgreements(t *testing.T) {
	const imsi = "999700000000001"

	partnerID := "p"
	visitor := &db.Subscriber{Imsi: imsi, ProfileID: "partner-profile", RoamingPartnerID: &partnerID}
	both := &db.Profile{ID: "partner-profile", Allow4G: true, Allow5G: true}
	failure := errors.New("database unavailable")
	now := time.Now()

	for _, tc := range []struct {
		name    string
		store   n26Store
		want    bool
		wantErr error
	}{
		{name: "home subscriber", store: n26Store{sub: &db.Subscriber{Imsi: "001010000000001"}}, want: true},
		{name: "subscriber row gone", store: n26Store{subErr: db.ErrNotFound}},
		{name: "lookup failure", store: n26Store{subErr: failure}, wantErr: failure},
		{name: "visitor of a partner with both agreements", store: n26Store{sub: visitor, partner: n26Partner(), profile: both}, want: true},
		{name: "partner gone", store: n26Store{sub: visitor, profile: both}},
		{name: "partner without a 4G agreement", store: n26Store{sub: visitor, partner: func() *db.RoamingPartner {
			p := n26Partner()
			p.DiameterPeerID = nil

			return p
		}(), profile: both}},
		{name: "partner without a 5G agreement", store: n26Store{sub: visitor, partner: func() *db.RoamingPartner {
			p := n26Partner()
			p.SEPPFQDN = nil

			return p
		}(), profile: both}},
		{name: "partner profile without 4G", store: n26Store{sub: visitor, partner: n26Partner(), profile: &db.Profile{Allow5G: true}}},
		{name: "partner profile without 5G", store: n26Store{sub: visitor, partner: n26Partner(), profile: &db.Profile{Allow4G: true}}},
		{name: "5G agreement being withdrawn", store: n26Store{sub: visitor, partner: func() *db.RoamingPartner {
			p := n26Partner()
			p.Withdrawals = []db.RoamingWithdrawal{{PartnerID: "p", Type: db.UERegistrationTypeAMF3GPPAccess, ExpiresAt: now.Add(time.Hour).Unix()}}

			return p
		}(), profile: both}},
		{name: "roamer being withdrawn", store: n26Store{sub: visitor, partner: func() *db.RoamingPartner {
			p := n26Partner()
			p.Withdrawals = []db.RoamingWithdrawal{{PartnerID: "p", IMSI: imsi, ExpiresAt: now.Add(time.Hour).Unix()}}

			return p
		}(), profile: both}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := NewN26(tc.store)
			n.now = func() time.Time { return now }

			got, err := n.Allowed(context.Background(), imsi)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}

			if got != tc.want {
				t.Fatalf("Allowed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEPSInterworkingFollowsTheHomeIwkEpsInd(t *testing.T) {
	serving := models.Snssai{Sst: 1, Sd: "010203"}

	sel := &wire.SmfSelectionSubscriptionData{SubscribedSnssaiInfos: map[string]wire.SnssaiInfo{
		"1-010203": {DnnInfos: []wire.DnnInfo{
			{Dnn: "internet", LboRoamingAllowed: true, IwkEpsInd: true},
			{Dnn: "iot", LboRoamingAllowed: true},
			{Dnn: "barred", LboRoamingAllowed: true, IwkEpsInd: true, DnnBarred: true},
		}},
	}}

	for _, tc := range []struct {
		name   string
		sel    *wire.SmfSelectionSubscriptionData
		snssai models.Snssai
		dnn    string
		want   bool
	}{
		{name: "subscribed", sel: sel, snssai: serving, dnn: "internet", want: true},
		{name: "indication absent", sel: sel, snssai: serving, dnn: "iot"},
		{name: "barred DNN", sel: sel, snssai: serving, dnn: "barred"},
		{name: "other slice", sel: sel, snssai: models.Snssai{Sst: 2}, dnn: "internet"},
		{name: "no SMF selection data", snssai: serving, dnn: "internet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EPSInterworkingSubscribed(tc.sel, tc.snssai, tc.dnn); got != tc.want {
				t.Fatalf("EPSInterworkingSubscribed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFiveGSInterworkingFollowsTheHomeAPNIndicator(t *testing.T) {
	with := func(modify func(*s6a.SubscriptionData)) *s6a.SubscriptionData {
		internet := fakehss.APN(1, "internet")
		internet.Interworking5GSIndicator = true

		sub := fakehss.Subscription(1, internet, fakehss.APN(2, "iot"))
		if modify != nil {
			modify(&sub)
		}

		return &sub
	}

	notAllowed := s6a.CNR5GCNotAllowed

	for _, tc := range []struct {
		name string
		home *s6a.SubscriptionData
		apn  string
		want bool
	}{
		{name: "subscribed", home: with(nil), apn: "Internet", want: true},
		{name: "indicator absent", home: with(nil), apn: "iot"},
		{name: "APN not subscribed", home: with(nil), apn: "corporate"},
		{name: "5GC not allowed", home: with(func(s *s6a.SubscriptionData) { s.CoreNetworkRestrictions = &notAllowed }), apn: "internet"},
		{name: "no home subscription", apn: "internet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FiveGSInterworkingSubscribed(tc.home, tc.apn); got != tc.want {
				t.Fatalf("FiveGSInterworkingSubscribed = %v, want %v", got, tc.want)
			}
		})
	}
}

type epsHomeFallback struct {
	sm *udm.SessionManagementSubscription
}

func (f epsHomeFallback) SessionManagement(context.Context, string) (*udm.SessionManagementSubscription, error) {
	return f.sm, nil
}

func (epsHomeFallback) FramedRoutes(context.Context, string, string) ([]netip.Prefix, error) {
	return nil, nil
}

func (epsHomeFallback) StaticIP(context.Context, string, string, bool) (netip.Addr, bool, error) {
	return netip.Addr{}, false, nil
}

func (epsHomeFallback) N26Allowed(context.Context, string) (bool, error) {
	return true, nil
}

func TestSessionsOfAVisitorBackInEPSKeepTheEPSHomeAPNs(t *testing.T) {
	f := newVSMFFixture(t)
	f.mustOpen(t, 5)

	eps := &udm.SessionManagementSubscription{Visitor: true, HomePLMN: vsmfHome.Model(), DNNs: []udm.DNNConfiguration{
		{Snssai: vsmfSnssai, DNN: "internet", Var5qi: 7},
		{Snssai: vsmfSnssai, DNN: "corporate", Var5qi: 8},
	}}

	sm, err := f.v.Subscriptions(epsHomeFallback{sm: eps}).SessionManagement(context.Background(), vsmfIMSI)
	if err != nil {
		t.Fatal(err)
	}

	internet, ok := sm.ForDNN(vsmfSnssai, "internet")
	if !ok || internet.Var5qi != 9 {
		t.Fatalf("internet = %+v, %v, want the 5G home data the session is bound to", internet, ok)
	}

	if _, ok := sm.ForDNN(vsmfSnssai, "corporate"); !ok {
		t.Fatalf("DNNs = %+v, want the EPS home APN that has no 5G session", sm.DNNs)
	}
}

type visitorLocal struct{}

func (visitorLocal) IsHomeSubscriber(context.Context, string) (bool, error) { return false, nil }

func (visitorLocal) AccessAndMobility(context.Context, string) (*udm.AccessAndMobilitySubscription, error) {
	return &udm.AccessAndMobilitySubscription{Visitor: true}, nil
}

func (visitorLocal) UEAMBR(context.Context, string) (models.Ambr, error) { return models.Ambr{}, nil }

func (visitorLocal) SessionManagement(context.Context, string) (*udm.SessionManagementSubscription, error) {
	return &udm.SessionManagementSubscription{Visitor: true}, nil
}

func (visitorLocal) FramedRoutes(context.Context, string, string) ([]netip.Prefix, error) {
	return nil, nil
}

func (visitorLocal) StaticIP(context.Context, string, string, bool) (netip.Addr, bool, error) {
	return netip.Addr{}, false, nil
}

type allowAll struct{}

func (allowAll) Allowed(context.Context, string) (bool, error) { return true, nil }

func TestSubscriptionsAskTheN26PolicyOnceSet(t *testing.T) {
	s := NewSubscriptions(visitorLocal{}, nil)

	if ok, err := s.N26Allowed(context.Background(), "999700000000001"); err != nil || ok {
		t.Fatalf("N26Allowed without a policy = %v, %v, want the home-only fallback", ok, err)
	}

	s.SetN26(allowAll{})

	if ok, err := s.N26Allowed(context.Background(), "999700000000001"); err != nil || !ok {
		t.Fatalf("N26Allowed with a policy = %v, %v", ok, err)
	}
}
