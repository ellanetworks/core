// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type changingResolver struct {
	mu      sync.Mutex
	addrs   map[string][]netip.Addr
	lookups atomic.Int32
}

func (r *changingResolver) set(host string, addrs ...netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.addrs == nil {
		r.addrs = map[string][]netip.Addr{}
	}

	r.addrs[host] = addrs
}

func (r *changingResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	r.lookups.Add(1)

	r.mu.Lock()
	defer r.mu.Unlock()

	addrs, ok := r.addrs[host]
	if !ok {
		return nil, errors.New("no such host")
	}

	return addrs, nil
}

func namedPeer(name string, port uint16) Peer {
	return Peer{ID: "hss", Name: name, Transports: []Transport{TransportTCP}, Dial: &Dial{Port: port}, Applications: []Application{sgdApp}}
}

func TestPeerNameIsResolvedOnEachConnectionAttempt(t *testing.T) {
	resolver := &changingResolver{}
	_, smsc, addr := pair(t, TransportTCP, func(cfg *Config) { cfg.Resolver = resolver })

	setPeers(t, smsc, namedPeer("hss.svc.local", addr.Port()))

	eventually(t, "a failed lookup", func() bool {
		s, ok := smsc.Peer("hss")
		return ok && strings.Contains(s.LastError, "resolve hss.svc.local")
	})

	before := resolver.lookups.Load()

	eventually(t, "another lookup", func() bool { return resolver.lookups.Load() > before })

	resolver.set("hss.svc.local", netip.MustParseAddr("::ffff:127.0.0.1"), addr.Addr())

	eventually(t, "the SMSC to reach the HSS by name", func() bool { return doOK(smsc, "hss") })

	if s, _ := smsc.Peer("hss"); s.RemoteAddr != addr.Addr() {
		t.Fatalf("status = %+v", s)
	}
}

func TestIncomingConnectionMatchesResolvedPeerName(t *testing.T) {
	resolver := &changingResolver{}
	resolver.set("smsc.svc.local", loopback2)

	hss := newTestNode(t, func() Config {
		cfg := testConfig("hss.example.org")
		cfg.Resolver = resolver

		return cfg
	}())

	addr := serveOn(t, hss, TransportTCP, loopback1)

	setPeers(t, hss, Peer{ID: "smsc", Name: "smsc.svc.local", Transports: []Transport{TransportTCP}, Dial: &Dial{Port: 1}, Applications: []Application{sgdApp}})

	eventually(t, "a lookup", func() bool { return resolver.lookups.Load() > 0 })

	smscCfg := testConfig("smsc.example.org")
	smscCfg.Identity.HostIPAddresses = []netip.Addr{loopback2}
	smsc := newTestNode(t, smscCfg)

	setPeers(t, smsc, Peer{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Transports: []Transport{TransportTCP}, Dial: &Dial{Port: addr.Port()}, Applications: []Application{sgdApp}})

	waitOpen(t, hss, "smsc")
}

func TestSetPeersRejectsInvalidPeerNames(t *testing.T) {
	n := newTestNode(t, testConfig("ella.example.org"))

	for _, tc := range []struct {
		name string
		peer Peer
		want string
	}{
		{"name and addresses", func() Peer {
			p := namedPeer("hss.example.org", 3868)
			p.Addresses = []netip.Addr{loopback2}

			return p
		}(), "both a name and addresses"},
		{"name without dial", func() Peer {
			p := namedPeer("hss.example.org", 3868)
			p.Dial = nil

			return p
		}(), "does not dial"},
		{"invalid name", namedPeer("hss..example.org", 3868), "invalid name"},
		{"no name or address", namedPeer("", 3868), "needs a name or at least one address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := n.SetPeers([]Peer{tc.peer})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("SetPeers = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSetPeersRejectsHostlessPeersSharingAName(t *testing.T) {
	n := newTestNode(t, testConfig("ella.example.org"))

	a := namedPeer("smsc.example.org", 3868)
	b := namedPeer("SMSC.example.org", 3869)
	b.ID = "other"

	if err := n.SetPeers([]Peer{a, b}); err == nil || !strings.Contains(err.Error(), "share the name") {
		t.Fatalf("SetPeers = %v", err)
	}
}

func TestIsValidHostname(t *testing.T) {
	for name, want := range map[string]bool{
		"smsc":                                 true,
		"smsc-0.smsc.ella.svc.cluster.local":   true,
		"SMSC.Example.org":                     true,
		"3com.example":                         true,
		"":                                     false,
		"10.0.0.1":                             false,
		"2001:db8::1":                          false,
		"smsc.":                                false,
		".smsc":                                false,
		"-smsc.example.org":                    false,
		"smsc-.example.org":                    false,
		"smsc_0.example.org":                   false,
		strings.Repeat("a", 64) + ".example":   false,
		strings.Repeat("abcdefghi.", 26) + "x": false,
	} {
		if got := IsValidHostname(name); got != want {
			t.Errorf("IsValidHostname(%q) = %v, want %v", name, got, want)
		}
	}
}
