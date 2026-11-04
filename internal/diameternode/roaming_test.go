// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
)

const (
	partnerRealm = "epc.mnc070.mcc999.3gppnetwork.org"
	partnerHSS   = "hss.epc.mnc070.mcc999.3gppnetwork.org"
	partnerDEA   = "dea.epc.mnc070.mcc999.3gppnetwork.org"
	roamerIMSI   = "999700000000001"
)

type fakeHPLMNPeer struct {
	node *diameter.Node
	ln   *sctp.Listener
	addr netip.AddrPort

	mu       sync.Mutex
	received []tgpp.Envelope
}

func startFakeHPLMN(t *testing.T, host string, apps []diameter.Application) *fakeHPLMNPeer {
	t.Helper()

	h := &fakeHPLMNPeer{}
	hss := diameter.Identity{OriginHost: partnerHSS, OriginRealm: partnerRealm}

	node, err := diameter.New(diameter.Config{
		Identity: diameter.Identity{
			OriginHost:      host,
			OriginRealm:     partnerRealm,
			HostIPAddresses: []netip.Addr{loopback},
			ProductName:     "fake-home-network",
		},
		Handler: diameter.HandlerFunc(func(_ context.Context, _ *diameter.Conn, req *diameter.Message) *diameter.Message {
			h.mu.Lock()
			h.received = append(h.received, tgpp.ParseEnvelope(req))
			h.mu.Unlock()

			if _, err := s6a.ParseAuthenticationInformationRequest(req); err != nil {
				return s6a.NewErrorAnswer(req, hss, err, s6a.Features{})
			}

			ans, err := s6a.NewAuthenticationInformationAnswer(req, hss, s6a.AuthenticationInformation{
				EUTRAN: []s6a.EUTRANVector{{ItemNumber: 1, RAND: make([]byte, 16), XRES: make([]byte, 8), AUTN: make([]byte, 16), KASME: make([]byte, 32)}},
			})
			if err != nil {
				return s6a.NewErrorAnswer(req, hss, err, s6a.Features{})
			}

			return ans
		}),
		AcceptUnknownPeers:      true,
		UnknownPeerApplications: apps,
	})
	if err != nil {
		t.Fatalf("new fake home network: %v", err)
	}

	var lc sctp.ListenConfig

	ln, err := lc.Listen(context.Background(), &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: loopback.AsSlice()}}, Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	a, ok := ln.Addr().(*sctp.SCTPAddr)
	if !ok {
		t.Fatalf("unexpected listener address %v", ln.Addr())
	}

	go func() { _ = node.Serve(diameter.NewSCTPListener(ln, nil)) }()

	h.node, h.ln, h.addr = node, ln, netip.AddrPortFrom(loopback, uint16(a.Port))

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = node.Shutdown(ctx)
		_ = ln.Close()
	})

	return h
}

func (h *fakeHPLMNPeer) envelopes() []tgpp.Envelope {
	h.mu.Lock()
	defer h.mu.Unlock()

	return slices.Clone(h.received)
}

type roamingSource struct {
	*settingsSource

	mu       sync.Mutex
	peers    []db.RoamingDiameterPeer
	partners []db.RoamingPartner
}

func (s *roamingSource) getPeers(context.Context) ([]diameternode.PeerConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return roaming.DiameterPeers(s.peers, s.partners)
}

func (s *roamingSource) setPartners(partners []db.RoamingPartner) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.partners = partners
}

func startRoamingManager(t *testing.T, home *fakeHPLMNPeer, identity string) (*diameternode.Manager, *roamingSource, chan struct{}) {
	t.Helper()

	peerID := "dea"
	source := &roamingSource{
		settingsSource: newSettingsSource(),
		peers:          []db.RoamingDiameterPeer{{ID: peerID, DiameterIdentity: identity, Address: home.addr.Addr().String(), Port: int(home.addr.Port())}},
		partners:       []db.RoamingPartner{{ID: "p1", Name: "partner", DiameterPeerID: &peerID, PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}}}},
	}

	link, wakeup := runManager(t, diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{}, zap.NewNop()))

	waitFor(t, "roaming peer up", func() bool {
		for _, p := range link.Peers() {
			if p.Role == diameternode.PeerRoleRoaming && p.State == diameter.PeerOpen {
				return true
			}
		}

		return false
	})

	return link, source, wakeup
}

func s6aClient(link *diameternode.Manager) *roaming.S6a {
	return roaming.NewS6a(func() roaming.DiameterNode {
		if node := link.Node(); node != nil {
			return node
		}

		return nil
	})
}

func TestRoamingHSSPeerNegotiatesS6aAndAnswersByRealm(t *testing.T) {
	requireSCTP(t)

	home := startFakeHPLMN(t, partnerHSS, []diameter.Application{{ID: s6a.ApplicationID, VendorID: tgpp.VendorID}})
	link, _, _ := startRoamingManager(t, home, partnerHSS)

	_, host, err := s6aClient(link).AuthenticationInformation(context.Background(), roaming.HPLMNPeer{Realm: partnerRealm}, roamerIMSI, tgpp.PLMN{MCC: "001", MNC: "01"}, nil)
	if err != nil {
		t.Fatalf("AIR: %v", err)
	}

	if host != partnerHSS {
		t.Fatalf("HSS = %q, want %s", host, partnerHSS)
	}

	env := home.envelopes()[0]
	if env.DestinationRealm != partnerRealm || env.DestinationHost != "" || env.Origin.OriginHost != ellaHost || env.Origin.OriginRealm != ellaRealm {
		t.Fatalf("AIR envelope %+v, want %s from %s without Destination-Host", env, partnerRealm, ellaHost)
	}
}

func TestRoamingRelayAgentCarriesRequestsForAnHSSBehindIt(t *testing.T) {
	requireSCTP(t)

	dea := startFakeHPLMN(t, partnerDEA, []diameter.Application{{ID: diameter.RelayApplicationID}, {ID: s6a.ApplicationID, VendorID: tgpp.VendorID}})
	link, _, _ := startRoamingManager(t, dea, "")

	client := s6aClient(link)
	visited := tgpp.PLMN{MCC: "001", MNC: "01"}

	if _, _, err := client.AuthenticationInformation(context.Background(), roaming.HPLMNPeer{Realm: partnerRealm}, roamerIMSI, visited, nil); err != nil {
		t.Fatalf("first AIR: %v", err)
	}

	_, _, err := client.AuthenticationInformation(context.Background(), roaming.HPLMNPeer{Realm: partnerRealm, HSS: partnerHSS}, roamerIMSI, visited, nil)

	var re *s6a.ResultError
	if !errors.As(err, &re) || re.Code != diameter.ResultUnableToDeliver || errors.Is(err, roaming.ErrHSSUnreachable) {
		t.Fatalf("AIR to an HSS behind the relay: %v, want the relay's own DIAMETER_UNABLE_TO_DELIVER answer", err)
	}

	if envs := dea.envelopes(); len(envs) != 1 || envs[0].DestinationRealm != partnerRealm {
		t.Fatalf("envelopes %+v, want the first AIR routed by realm to the relay", envs)
	}
}

func TestRoamingRoutesFollowThePartnerPLMNs(t *testing.T) {
	requireSCTP(t)

	home := startFakeHPLMN(t, partnerHSS, []diameter.Application{{ID: s6a.ApplicationID, VendorID: tgpp.VendorID}})
	link, source, wakeup := startRoamingManager(t, home, partnerHSS)

	peer := "dea"
	source.setPartners([]db.RoamingPartner{{ID: "p1", Name: "partner", DiameterPeerID: &peer, PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}, {Mcc: "998", Mnc: "01"}}}})
	poke(wakeup)

	waitFor(t, "the new partner realm routed", func() bool {
		node := link.Node()
		if node == nil {
			return false
		}

		return slices.ContainsFunc(node.Routes(), func(r diameter.RouteStatus) bool {
			return r.Realm == "epc.mnc001.mcc998.3gppnetwork.org" && r.Application == s6a.ApplicationID && len(r.Peers) == 1
		})
	})

	if !slices.ContainsFunc(link.Peers(), func(s diameternode.PeerStatus) bool {
		return s.Role == diameternode.PeerRoleRoaming && s.State == diameter.PeerOpen
	}) {
		t.Fatal("changing the partner routes dropped the roaming peer")
	}
}
