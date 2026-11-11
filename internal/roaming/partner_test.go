// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming_test

import (
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/roaming"
)

func TestDiameterPeersRouteEveryPartnerPLMNToItsPeer(t *testing.T) {
	dea := "dea"
	peers := []db.RoamingDiameterPeer{
		{ID: dea, DiameterIdentity: "dea.ipx.example.org", Address: "192.0.2.10", Port: 3868},
		{ID: "spare", Address: "192.0.2.11", Port: 3869},
	}
	partners := []db.RoamingPartner{
		{ID: "p1", Name: "beta", DiameterPeerID: &dea, PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}, {Mcc: "998", Mnc: "001"}}},
		{ID: "p2", Name: "alpha", DiameterPeerID: &dea, PLMNs: []db.RoamingPLMN{{Mcc: "997", Mnc: "12"}}},
		{ID: "p3", Name: "five-g-only", PLMNs: []db.RoamingPLMN{{Mcc: "996", Mnc: "01"}}},
	}

	configs, err := roaming.DiameterPeers(peers, partners)
	if err != nil {
		t.Fatal(err)
	}

	if len(configs) != 2 {
		t.Fatalf("configs = %+v, want one per roaming Diameter peer", configs)
	}

	got := configs[0]
	want := []diameter.Route{
		{Realm: "epc.mnc001.mcc998.3gppnetwork.org", Application: s6a.ApplicationID},
		{Realm: "epc.mnc012.mcc997.3gppnetwork.org", Application: s6a.ApplicationID},
		{Realm: "epc.mnc070.mcc999.3gppnetwork.org", Application: s6a.ApplicationID},
	}

	if got.ID != "roaming-dea" || got.Role != diameternode.PeerRoleRoaming || got.Host != "dea.ipx.example.org" ||
		got.Address != "192.0.2.10" || got.Port != 3868 || !slices.Equal(got.Routes, want) ||
		!slices.Equal(got.Applications, diameternode.RoamingApplications) {
		t.Fatalf("config = %+v, want routes %+v", got, want)
	}

	if len(configs[1].Routes) != 0 {
		t.Fatalf("a peer without partners has routes %+v", configs[1].Routes)
	}
}

func TestPartnerOfIMSI(t *testing.T) {
	partners := []db.RoamingPartner{
		{ID: "p1", PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}}},
		{ID: "p2", PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "071"}}},
	}

	if p, plmn, ok := roaming.PartnerOfIMSI(partners, "999071000000001"); !ok || p.ID != "p2" || plmn.Mnc != "071" {
		t.Fatalf("PartnerOfIMSI = %+v %+v %v, want p2 with MNC 071", p, plmn, ok)
	}

	if _, _, ok := roaming.PartnerOfIMSI(partners, "001010000000001"); ok {
		t.Fatal("an IMSI outside every partner PLMN matched a partner")
	}
}

func TestConnectedS6aPartnersCountsPartnersOfOpenPeers(t *testing.T) {
	dea, spare, down := "dea", "spare", "down"
	partners := []db.RoamingPartner{
		{ID: "p1", DiameterPeerID: &dea},
		{ID: "p2", DiameterPeerID: &dea},
		{ID: "p3", DiameterPeerID: &down},
		{ID: "p4"},
		{ID: "p5", DiameterPeerID: &spare},
	}
	peers := []diameternode.PeerStatus{
		{ID: roaming.DiameterPeerID(dea), Role: diameternode.PeerRoleRoaming, State: diameter.PeerOpen},
		{ID: roaming.DiameterPeerID(down), Role: diameternode.PeerRoleRoaming, State: diameter.PeerDown},
		{ID: roaming.DiameterPeerID(spare), Role: diameternode.PeerRoleSMSC, State: diameter.PeerOpen},
	}

	if got := roaming.ConnectedS6aPartners(partners, peers); got != 2 {
		t.Fatalf("connected partners = %d, want 2", got)
	}
}
