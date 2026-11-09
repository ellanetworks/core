// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode_test

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/diameternode"
	"go.uber.org/zap"
)

var anyLoopback = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

type imsPeersSource struct {
	mu       sync.Mutex
	restrict bool
	peers    []diameternode.IMSPeer
	err      error
}

func newIMSPeersSource(restrict bool, peers ...diameternode.IMSPeer) *imsPeersSource {
	return &imsPeersSource{restrict: restrict, peers: peers}
}

func (s *imsPeersSource) set(peers ...diameternode.IMSPeer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.peers = peers
}

func (s *imsPeersSource) setRestrict(restrict bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.restrict = restrict
}

func (s *imsPeersSource) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.err = err
}

func (s *imsPeersSource) get(context.Context) (diameternode.IMSPeerPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return diameternode.IMSPeerPolicy{}, s.err
	}

	return diameternode.IMSPeerPolicy{Restrict: s.restrict, Peers: append([]diameternode.IMSPeer(nil), s.peers...)}, nil
}

func inboundPeers(link *diameternode.Manager) []diameternode.PeerStatus {
	var out []diameternode.PeerStatus

	for _, p := range link.Peers() {
		if p.Inbound {
			out = append(out, p)
		}
	}

	return out
}

func TestOpenListenerLabelsInboundPeersUnknown(t *testing.T) {
	requireSCTP(t)

	port := freePort(t)
	link, _ := startListeningManager(t, newSettingsSource(), diameternode.ListenConfig{Address: loopback.String(), Port: port})

	ims := newDialingNode(t, "scscf.ims.mnc001.mcc001.3gppnetwork.org", loopback2)
	dialEllaFor(t, ims, port, diameter.TransportSCTP, imsApplications)

	waitFor(t, "inbound peer open", func() bool {
		return imsPeer(link).State == diameter.PeerOpen
	})

	if role := imsPeer(link).Role; role != diameternode.PeerRoleUnknown {
		t.Fatalf("role = %q, want %q", role, diameternode.PeerRoleUnknown)
	}
}

func TestIMSPeerAllowlist(t *testing.T) {
	requireSCTP(t)

	const (
		allowedHost  = "scscf.ims.mnc001.mcc001.3gppnetwork.org"
		intruderHost = "intruder.example.org"
	)

	port := freePort(t)
	source := newSettingsSource()
	peers := newIMSPeersSource(true, diameternode.IMSPeer{Name: "scscf", Host: allowedHost, Prefixes: anyLoopback})

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: loopback.String(), Port: port}, zap.NewNop())
	link.SetIMSPeersSource(peers.get)

	_, wakeup := runManager(t, link)

	allowed := newDialingNode(t, allowedHost, loopback2)
	dialEllaFor(t, allowed, port, diameter.TransportSCTP, imsApplications)

	waitFor(t, "allowed IMS peer open", func() bool {
		return ellaStatus(allowed).State == diameter.PeerOpen
	})

	if got := inboundPeers(link); len(got) != 1 || got[0].Role != diameternode.PeerRoleIMS {
		t.Fatalf("inbound peers = %+v, want one IMS peer", got)
	}

	intruder := newDialingNode(t, intruderHost, netip.MustParseAddr("127.0.0.4"))
	dialEllaFor(t, intruder, port, diameter.TransportSCTP, imsApplications)

	time.Sleep(500 * time.Millisecond)

	if st := ellaStatus(intruder).State; st == diameter.PeerOpen {
		t.Fatalf("intruder state = %s, want rejected", st)
	}

	peers.set(diameternode.IMSPeer{Name: "other", Realm: "ims.example.net", Prefixes: anyLoopback})

	wakeup <- struct{}{}

	waitFor(t, "removed IMS peer disconnected", func() bool {
		return len(inboundPeers(link)) == 0 && ellaStatus(allowed).State != diameter.PeerOpen
	})
}

func TestIMSPeerAllowlistMatchesTheSourceAddress(t *testing.T) {
	requireSCTP(t)

	port := freePort(t)
	source := newSettingsSource()
	peers := newIMSPeersSource(true, diameternode.IMSPeer{Name: "site-a", Realm: smscRealm, Prefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.2/32")}})

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: loopback.String(), Port: port}, zap.NewNop())
	link.SetIMSPeersSource(peers.get)

	_, wakeup := runManager(t, link)

	inside := newDialingNode(t, "scscf-a.example.org", loopback2)
	dialEllaFor(t, inside, port, diameter.TransportSCTP, imsApplications)

	waitFor(t, "peer inside the prefix open", func() bool {
		return ellaStatus(inside).State == diameter.PeerOpen
	})

	outside := newDialingNode(t, "scscf-b.example.org", netip.MustParseAddr("127.0.0.4"))
	dialEllaFor(t, outside, port, diameter.TransportSCTP, imsApplications)

	time.Sleep(500 * time.Millisecond)

	if st := ellaStatus(outside).State; st == diameter.PeerOpen {
		t.Fatalf("peer outside the prefix state = %s, want rejected", st)
	}

	if got := inboundPeers(link); len(got) != 1 || got[0].Role != diameternode.PeerRoleIMS || got[0].Host != "scscf-a.example.org" {
		t.Fatalf("inbound peers = %+v, want only scscf-a as an IMS peer", got)
	}

	peers.set(diameternode.IMSPeer{Name: "site-a", Realm: smscRealm, Prefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.4/32")}})

	wakeup <- struct{}{}

	waitFor(t, "the peer whose address is no longer allowed disconnected", func() bool {
		for _, p := range inboundPeers(link) {
			if p.Host == "scscf-a.example.org" {
				return false
			}
		}

		return ellaStatus(inside).State != diameter.PeerOpen
	})
}

func TestIMSPeerMatches(t *testing.T) {
	remote := netip.MustParseAddr("10.1.2.3")

	for name, tc := range map[string]struct {
		peer diameternode.IMSPeer
		want bool
	}{
		"host":                      {diameternode.IMSPeer{Host: "scscf.example.org", Prefixes: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}}, true},
		"host without prefixes":     {diameternode.IMSPeer{Host: "scscf.example.org"}, false},
		"host and other realm":      {diameternode.IMSPeer{Host: "scscf.example.org", Realm: "other.example.org", Prefixes: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}}, false},
		"realm and prefix":          {diameternode.IMSPeer{Realm: "example.org", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}, true},
		"realm and other prefix":    {diameternode.IMSPeer{Realm: "example.org", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.2.0.0/16")}}, false},
		"host and one of prefixes":  {diameternode.IMSPeer{Host: "scscf.example.org", Prefixes: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("10.1.2.3/32")}}, true},
		"prefix without identity":   {diameternode.IMSPeer{Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}, false},
		"IPv6 prefix, IPv4 address": {diameternode.IMSPeer{Realm: "example.org", Prefixes: []netip.Prefix{netip.MustParsePrefix("::/0")}}, false},
	} {
		if got := tc.peer.Matches("SCSCF.example.org", "example.org", remote); got != tc.want {
			t.Fatalf("%s: Matches = %v, want %v", name, got, tc.want)
		}
	}

	mapped := netip.MustParseAddr("::ffff:10.1.2.3")
	if !(diameternode.IMSPeer{Realm: "example.org", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}).Matches("h", "example.org", mapped) {
		t.Fatal("an IPv4-mapped IPv6 source address did not match its IPv4 prefix")
	}
}

func TestIMSPeerPolicyOpenAdmitsAnyPeerAndLabelsAllowedOnes(t *testing.T) {
	requireSCTP(t)

	const allowedHost = "scscf.ims.mnc001.mcc001.3gppnetwork.org"

	port := freePort(t)
	source := newSettingsSource()
	peers := newIMSPeersSource(false, diameternode.IMSPeer{Name: "scscf", Host: allowedHost, Prefixes: anyLoopback})

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: loopback.String(), Port: port}, zap.NewNop())
	link.SetIMSPeersSource(peers.get)

	_, wakeup := runManager(t, link)

	allowed := newDialingNode(t, allowedHost, loopback2)
	dialEllaFor(t, allowed, port, diameter.TransportSCTP, imsApplications)

	other := newDialingNode(t, "other.example.org", netip.MustParseAddr("127.0.0.4"))
	dialEllaFor(t, other, port, diameter.TransportSCTP, imsApplications)

	waitFor(t, "both peers open", func() bool {
		return ellaStatus(allowed).State == diameter.PeerOpen && ellaStatus(other).State == diameter.PeerOpen
	})

	roles := map[string]string{}
	for _, p := range inboundPeers(link) {
		roles[p.Host] = p.Role
	}

	if roles[allowedHost] != diameternode.PeerRoleIMS || roles["other.example.org"] != diameternode.PeerRoleUnknown {
		t.Fatalf("roles = %v", roles)
	}

	peers.setError(errors.New("database unavailable"))
	peers.setRestrict(true)

	wakeup <- struct{}{}

	time.Sleep(500 * time.Millisecond)

	if ellaStatus(other).State != diameter.PeerOpen || link.Node() == nil {
		t.Fatal("a failed read of the allowed IMS peers changed the running node")
	}

	peers.setError(nil)

	wakeup <- struct{}{}

	waitFor(t, "the peer that matches no entry disconnected", func() bool {
		return ellaStatus(other).State != diameter.PeerOpen
	})

	if ellaStatus(allowed).State != diameter.PeerOpen {
		t.Fatal("the allowed IMS peer was disconnected")
	}

	peers.set()

	wakeup <- struct{}{}

	waitFor(t, "an empty restricted list rejects every peer", func() bool {
		return len(inboundPeers(link)) == 0 && ellaStatus(allowed).State != diameter.PeerOpen
	})
}
