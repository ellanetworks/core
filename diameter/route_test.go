// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"
)

const hubHost = "smsc.example.org"

func newHub(t *testing.T, kind Transport) (*Node, netip.AddrPort) {
	t.Helper()

	hub := newClient(t, func(cfg *Config) {
		cfg.Identity.OriginHost = hubHost
		cfg.AcceptUnknownPeers = true
		cfg.UnknownPeerApplications = []Application{sgdApp, otherApp}
	})

	return hub, serveOn(t, hub, kind, loopback1)
}

func (s *server) dialIn(t *testing.T, kind Transport, hub netip.AddrPort, apps ...Application) {
	t.Helper()

	if len(apps) == 0 {
		apps = []Application{sgdApp}
	}

	setPeers(t, s.node, Peer{
		ID: hubHost, Host: hubHost, Addresses: []netip.Addr{hub.Addr()}, Transports: []Transport{kind},
		Applications: apps, Dial: &Dial{Port: hub.Port()},
	})
}

func implicitHosts(n *Node, realm string, app uint32) map[string]PeerState {
	hosts := make(map[string]PeerState)

	for _, r := range n.Routes() {
		if r.Realm != realm || r.Application != app {
			continue
		}

		for _, p := range r.Peers {
			if p.Implicit {
				hosts[p.Host] = p.State
			}
		}
	}

	return hosts
}

func waitImplicit(t *testing.T, n *Node, hosts ...string) {
	t.Helper()

	eventually(t, fmt.Sprintf("implicit routes to %v", hosts), func() bool {
		got := implicitHosts(n, routeRealm, testAppID)

		for _, h := range hosts {
			if got[h] != PeerOpen {
				return false
			}
		}

		return true
	})
}

func TestSendReachesAnAcceptedPeerByRealm(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			hub, addr := newHub(t, kind)

			hss := newServer(t, kind, "hss1.example.org", loopback2, nil)
			hss.dialIn(t, kind, addr)
			waitImplicit(t, hub, hss.host)

			ans := send(t, hub, routed())
			if got := answeredBy(t, ans); got != hss.host {
				t.Fatalf("answered by %s", got)
			}

			if _, ok := (<-hss.seen).Find(AVPDestinationHost, 0); ok {
				t.Fatal("realm-routed request carries a Destination-Host")
			}
		})
	}
}

func TestSendSkipsAcceptedPeersOfAnotherRealmOrApplication(t *testing.T) {
	kind := TransportTCP
	hub, addr := newHub(t, kind)

	elsewhere := newServerIn(t, kind, "hss1.other.org", "other.org", loopback2, []Application{sgdApp}, nil)
	elsewhere.dialIn(t, kind, addr)

	otherApplication := newServerIn(t, kind, "hss2.example.org", routeRealm, loopback3, []Application{otherApp}, nil)
	otherApplication.dialIn(t, kind, addr, otherApp)

	eventually(t, "both peers to open", func() bool { return len(hub.Peers()) == 2 })

	if _, err := hub.Send(context.Background(), routed(), FailFast()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Send = %v", err)
	}

	if elsewhere.calls.Load() != 0 || otherApplication.calls.Load() != 0 {
		t.Fatalf("calls %d/%d", elsewhere.calls.Load(), otherApplication.calls.Load())
	}
}

func TestSendRotatesBetweenAcceptedPeers(t *testing.T) {
	kind := TransportTCP
	hub, addr := newHub(t, kind)

	a := newServer(t, kind, "hss1.example.org", loopback2, nil)
	b := newServer(t, kind, "hss2.example.org", loopback3, nil)
	a.dialIn(t, kind, addr)
	b.dialIn(t, kind, addr)
	waitImplicit(t, hub, a.host, b.host)

	for range 6 {
		send(t, hub, routed())
	}

	if a.calls.Load() != 3 || b.calls.Load() != 3 {
		t.Fatalf("shared %d/%d", a.calls.Load(), b.calls.Load())
	}
}

func TestSendRetriesAnotherAcceptedPeerOnTooBusyAndUnableToDeliver(t *testing.T) {
	for _, code := range []uint32{ResultTooBusy, ResultUnableToDeliver} {
		t.Run(ResultName(code), func(t *testing.T) {
			kind := TransportTCP
			hub, addr := newHub(t, kind)

			refusing := newServer(t, kind, "hss1.example.org", loopback2, func(_ *server, c *Conn, req *Message) *Message {
				return c.Answer(req, code)
			})
			good := newServer(t, kind, "hss2.example.org", loopback3, nil)
			refusing.dialIn(t, kind, addr)
			good.dialIn(t, kind, addr)
			waitImplicit(t, hub, refusing.host, good.host)

			if got := answeredBy(t, send(t, hub, routed())); got != good.host {
				t.Fatalf("answered by %s", got)
			}

			first, second := <-refusing.seen, <-good.seen
			if first.EndToEndID != second.EndToEndID || first.Flags&FlagRetransmit != 0 || second.Flags&FlagRetransmit == 0 {
				t.Fatalf("End-to-End %d then %d, flags %#x then %#x", first.EndToEndID, second.EndToEndID, first.Flags, second.Flags)
			}
		})
	}
}

func TestSendReturnsTheLastRefusalFromAcceptedPeersWithoutWaiting(t *testing.T) {
	kind := TransportTCP
	hub, addr := newHub(t, kind)

	a := newServer(t, kind, "hss1.example.org", loopback2, busy)
	b := newServer(t, kind, "hss2.example.org", loopback3, busy)
	a.dialIn(t, kind, addr)
	b.dialIn(t, kind, addr)
	waitImplicit(t, hub, a.host, b.host)

	start := time.Now()

	ans := send(t, hub, routed())
	if resultCode(t, ans) != ResultTooBusy || a.calls.Load() != 1 || b.calls.Load() != 1 || time.Since(start) > time.Second {
		t.Fatalf("answer %d after %d/%d calls in %s", resultCode(t, ans), a.calls.Load(), b.calls.Load(), time.Since(start))
	}
}

func TestSendFailsOverBetweenAcceptedPeers(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			hub, addr := newHub(t, kind)

			var once sync.Once

			dropping := newServer(t, kind, "hss1.example.org", loopback2, func(_ *server, c *Conn, req *Message) *Message {
				dropped := false

				once.Do(func() {
					c.abort("test failover")

					dropped = true
				})

				if dropped {
					return nil
				}

				return c.Answer(req, ResultSuccess)
			})
			good := newServer(t, kind, "hss2.example.org", loopback3, nil)
			dropping.dialIn(t, kind, addr)
			good.dialIn(t, kind, addr)
			waitImplicit(t, hub, dropping.host, good.host)

			if got := answeredBy(t, send(t, hub, routed())); got != good.host {
				t.Fatalf("answered by %s", got)
			}

			first, second := <-dropping.seen, <-good.seen
			if first.EndToEndID != second.EndToEndID || second.Flags&FlagRetransmit == 0 {
				t.Fatalf("End-to-End %d then %d, flags %#x", first.EndToEndID, second.EndToEndID, second.Flags)
			}
		})
	}
}

func TestSendTriesStaticRoutesBeforeAcceptedPeers(t *testing.T) {
	kind := TransportTCP
	hub, addr := newHub(t, kind)

	static := newServer(t, kind, "hss1.example.org", loopback2, nil)
	accepted := newServer(t, kind, "hss2.example.org", loopback3, nil)

	setPeers(t, hub, static.peer(kind, 10))
	accepted.dialIn(t, kind, addr)
	waitOpen(t, hub, static.host)
	waitImplicit(t, hub, accepted.host)

	for range 4 {
		if got := answeredBy(t, send(t, hub, routed())); got != static.host {
			t.Fatalf("answered by %s", got)
		}
	}

	routes := hub.Routes()
	if len(routes) != 1 || len(routes[0].Peers) != 2 ||
		routes[0].Peers[0].ID != static.host || routes[0].Peers[0].Host != static.host || routes[0].Peers[0].Implicit ||
		routes[0].Peers[1].ID != "" || routes[0].Peers[1].Host != accepted.host || !routes[0].Peers[1].Implicit {
		t.Fatalf("Routes = %+v", routes)
	}
}

func TestSendIgnoresConfiguredPeersWithoutARouteInTheRealm(t *testing.T) {
	kind := TransportTCP
	hub, _ := newHub(t, kind)

	configured := newServer(t, kind, "hss1.example.org", loopback2, nil)
	p := configured.peer(kind, 0)
	p.Routes = nil

	setPeers(t, hub, p)
	waitOpen(t, hub, configured.host)

	if _, err := hub.Send(context.Background(), routed(), FailFast()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Send = %v", err)
	}

	if configured.calls.Load() != 0 {
		t.Fatalf("configured peer got %d requests", configured.calls.Load())
	}
}

func TestSendKeepsAnUnknownDestinationHostOffAcceptedPeers(t *testing.T) {
	kind := TransportTCP
	hub, addr := newHub(t, kind)

	hss := newServer(t, kind, "hss1.example.org", loopback2, nil)
	hss.dialIn(t, kind, addr)
	waitImplicit(t, hub, hss.host)

	start := time.Now()

	_, err := hub.Send(context.Background(), routed(UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "mme2.example.org")))
	if !errors.Is(err, ErrUnableToDeliver) || time.Since(start) > 100*time.Millisecond {
		t.Fatalf("Send = %v after %s", err, time.Since(start))
	}

	if hss.calls.Load() != 0 {
		t.Fatalf("accepted peer got %d requests", hss.calls.Load())
	}

	if got := answeredBy(t, send(t, hub, routed(UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, hss.host)))); got != hss.host {
		t.Fatalf("answered by %s", got)
	}
}

func TestSendWaitsForAnAcceptedPeerUnlessFailFast(t *testing.T) {
	kind := TransportTCP
	hub, addr := newHub(t, kind)

	start := time.Now()
	if _, err := hub.Send(context.Background(), routed(), FailFast()); !errors.Is(err, ErrNotConnected) || time.Since(start) > 100*time.Millisecond {
		t.Fatalf("FailFast Send = %v after %s", err, time.Since(start))
	}

	short, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if _, err := hub.Send(short, routed()); !errors.Is(err, ErrNotConnected) || !isTimeout(err) {
		t.Fatalf("Send without a peer = %v", err)
	}

	hss := newServer(t, kind, "hss1.example.org", loopback2, nil)

	type result struct {
		ans *Message
		err error
	}

	done := make(chan result, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		ans, err := hub.Send(ctx, routed())
		done <- result{ans, err}
	}()

	time.Sleep(100 * time.Millisecond)
	hss.dialIn(t, kind, addr)

	r := <-done
	if r.err != nil {
		t.Fatalf("Send = %v", r.err)
	}

	if got := answeredBy(t, r.ans); got != hss.host {
		t.Fatalf("answered by %s", got)
	}
}

func TestImplicitRoutesFollowTheConnection(t *testing.T) {
	kind := TransportTCP
	hub, addr := newHub(t, kind)

	hss := newServer(t, kind, "hss1.example.org", loopback2, nil)
	hss.dialIn(t, kind, addr, sgdApp, otherApp)
	waitImplicit(t, hub, hss.host)

	if got := implicitHosts(hub, routeRealm, otherAppID); got[hss.host] != PeerOpen {
		t.Fatalf("no implicit route for the second application: %+v", hub.Routes())
	}

	send(t, hub, routed())

	setPeers(t, hss.node)

	eventually(t, "implicit routes to be dropped", func() bool { return len(hub.Routes()) == 0 })

	hub.mu.Lock()
	defer hub.mu.Unlock()

	if len(hub.rotation) != 0 {
		t.Fatalf("rotation kept %v", hub.rotation)
	}
}
