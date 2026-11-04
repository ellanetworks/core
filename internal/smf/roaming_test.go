// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehss"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/udm"
)

type visitorSubscriptions struct {
	*fakeStore
}

func (v visitorSubscriptions) SessionManagement(ctx context.Context, imsi string) (*udm.SessionManagementSubscription, error) {
	sm, err := v.fakeStore.SessionManagement(ctx, imsi)
	if err != nil {
		return nil, err
	}

	sm.Visitor = true

	return sm, nil
}

func (visitorSubscriptions) IsHomeSubscriber(context.Context, string) (bool, error) {
	return false, nil
}

func (visitorSubscriptions) AccessAndMobility(context.Context, string) (*udm.AccessAndMobilitySubscription, error) {
	return &udm.AccessAndMobilitySubscription{Allow4G: true, Visitor: true}, nil
}

func (visitorSubscriptions) UEAMBR(context.Context, string) (models.Ambr, error) {
	return models.Ambr{}, nil
}

type homeSubscriptions map[string]*s6a.SubscriptionData

var visitorHomePLMN = models.PlmnID{Mcc: "001", Mnc: "01"}

func (h homeSubscriptions) VisitorHome(imsi string) (roaming.HomeData, bool) {
	s, ok := h[imsi]
	return roaming.HomeData{PLMN: visitorHomePLMN, Subscription: s}, ok
}

func visitorSMF(homes homeSubscriptions) (*smf.SMF, *fakePCF) {
	store, upf := epsTestSMF()
	pcf := &fakePCF{policy: epsPolicy()}
	store.pcf = pcf

	subs := roaming.NewSubscriptions(visitorSubscriptions{store}, homes)

	return smf.New(pcf, store, upf, &fakeAMF{}, smf.WithSubscriptions(subs)), pcf
}

func TestVisitorEPSSessionUsesTheMergedSubscription(t *testing.T) {
	home := fakehss.Subscription(1, fakehss.APN(1, "internet"))
	s, pcf := visitorSMF(homeSubscriptions{"001010000000001": &home})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatal(err)
	}

	pcf.mu.Lock()
	c, ok := pcf.associations[bearer.Ref]
	pcf.mu.Unlock()

	if !ok {
		t.Fatal("no policy association for the visitor session")
	}

	if c.Subscribed.SessionAMBR.Uplink.Bps() != 50_000_000 || c.Subscribed.SessionAMBR.Downlink.Bps() != 150_000_000 {
		t.Fatalf("subscribed Session-AMBR = %v, want min(home APN-AMBR 50/150 Mbps, local 1 Gbps)", c.Subscribed.SessionAMBR)
	}

	if c.Subscribed.Var5qi != 9 || c.Subscribed.Arp.PriorityLevel != 1 {
		t.Fatalf("subscribed 5QI/ARP = %d/%d, want the local 9/1", c.Subscribed.Var5qi, c.Subscribed.Arp.PriorityLevel)
	}
}

func TestVisitorEPSSessionWithoutAHomeSubscriptionIsRefused(t *testing.T) {
	s, _ := visitorSMF(homeSubscriptions{})

	_, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if !errors.Is(err, roaming.ErrNoHomeSubscription) {
		t.Fatalf("error = %v, want %v", err, roaming.ErrNoHomeSubscription)
	}
}

func TestVisitorEPSSessionForAnAPNTheHomeNetworkDoesNotSubscribeIsRefused(t *testing.T) {
	home := fakehss.Subscription(1, fakehss.APN(1, "corporate"))
	s, _ := visitorSMF(homeSubscriptions{"001010000000001": &home})

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err == nil {
		t.Fatal("the SMF opened a visitor session the home subscription does not allow")
	}
}

func TestVisitorUsageIsRecordedWithItsHomePLMN(t *testing.T) {
	home := fakehss.Subscription(1, fakehss.APN(1, "internet"))
	store, upf := epsTestSMF()
	pcf := &fakePCF{policy: epsPolicy()}
	store.pcf = pcf
	s := smf.New(pcf, store, upf, &fakeAMF{}, smf.WithSubscriptions(roaming.NewSubscriptions(visitorSubscriptions{store}, homeSubscriptions{"001010000000001": &home})))

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatal(err)
	}

	if fourG, fiveG := s.SessionCountByRAT(); fourG != (smf.SessionCount{Roaming: 1}) || fiveG != (smf.SessionCount{}) {
		t.Fatalf("session counts = 4G %+v, 5G %+v; want one 4G roaming session", fourG, fiveG)
	}

	upf.count(s.GetSession(bearer.Ref).PFCPContext.SEID, 700, 900)
	s.CollectUsageForTest(context.Background())

	store.mu.Lock()
	defer store.mu.Unlock()

	want := models.SubscriberUsage{EpochDay: today(), IMSI: "001010000000001", HomePLMN: visitorHomePLMN, UplinkVolume: 700, DownlinkVolume: 900}
	if len(store.usageRows) != 1 || store.usageRows[0] != want {
		t.Fatalf("usage = %+v, want [%+v]", store.usageRows, want)
	}
}

func TestHomeUsageIsRecordedWithoutAHomePLMN(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)

	if fourG, fiveG := s.SessionCountByRAT(); fourG != (smf.SessionCount{}) || fiveG != (smf.SessionCount{Home: 1}) {
		t.Fatalf("session counts = 4G %+v, 5G %+v; want one 5G home session", fourG, fiveG)
	}

	upf.count(sc.PFCPContext.SEID, 1, 2)
	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageRows) != 1 || store.usageRows[0].Roaming() {
		t.Fatalf("usage = %+v, want one home usage row", store.usageRows)
	}
}
