// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const routeRealm = "epc.example.org"

var loopback3 = netip.MustParseAddr("127.0.0.3")

type server struct {
	node  *Node
	host  string
	addr  netip.AddrPort
	calls atomic.Int32
	seen  chan *Message
}

func newServer(t *testing.T, kind Transport, host string, ip netip.Addr, handle func(s *server, c *Conn, req *Message) *Message) *server {
	t.Helper()

	s := &server{host: host, seen: make(chan *Message, 64)}

	cfg := testConfig(host)
	cfg.Identity.OriginRealm = routeRealm
	cfg.Identity.HostIPAddresses = []netip.Addr{ip}
	cfg.AcceptUnknownPeers = true
	cfg.UnknownPeerApplications = []Application{sgdApp}
	cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
		s.calls.Add(1)

		select {
		case s.seen <- req:
		default:
		}

		if handle != nil {
			return handle(s, c, req)
		}

		return c.Answer(req, ResultSuccess)
	})

	s.node = newTestNode(t, cfg)
	s.addr = serveOn(t, s.node, kind, ip)

	return s
}

func (s *server) peer(kind Transport, priority int) Peer {
	return Peer{
		ID:           s.host,
		Host:         s.host,
		Addresses:    []netip.Addr{s.addr.Addr()},
		Transports:   []Transport{kind},
		Applications: []Application{sgdApp},
		Routes:       []Route{{Realm: routeRealm, Application: testAppID, Priority: priority}},
		Dial:         &Dial{Port: s.addr.Port()},
	}
}

func newClient(t *testing.T, opts ...func(*Config)) *Node {
	t.Helper()

	cfg := testConfig("ims.example.org")
	for _, opt := range opts {
		opt(&cfg)
	}

	return newTestNode(t, cfg)
}

func setPeers(t *testing.T, n *Node, peers ...Peer) {
	t.Helper()

	if err := n.SetPeers(peers); err != nil {
		t.Fatal(err)
	}
}

func waitOpen(t *testing.T, n *Node, ids ...string) {
	t.Helper()

	eventually(t, fmt.Sprintf("peers %v to open", ids), func() bool {
		for _, id := range ids {
			if s, ok := n.Peer(id); !ok || s.State != PeerOpen {
				return false
			}
		}

		return true
	})
}

func routed(extra ...AVP) *Message {
	m := request()
	m.AVPs = append(m.AVPs,
		UTF8String(AVPSessionID, AVPFlagMandatory, 0, "ims.example.org;1;1"),
		UTF8String(AVPDestinationRealm, AVPFlagMandatory, 0, routeRealm),
	)
	m.AVPs = append(m.AVPs, extra...)

	return m
}

func send(t *testing.T, n *Node, req *Message, opts ...RequestOption) *Message {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	ans, err := n.Send(ctx, req, opts...)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	return ans
}

func answeredBy(t *testing.T, ans *Message) string {
	t.Helper()

	a, ok := ans.Find(AVPOriginHost, 0)
	if !ok {
		t.Fatal("answer without Origin-Host")
	}

	return a.UTF8String()
}

func busy(_ *server, c *Conn, req *Message) *Message {
	return c.Answer(req, ResultTooBusy)
}

func TestSendPrefersTheLowestPriority(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			primary := newServer(t, kind, "hss1.example.org", loopback2, nil)
			secondary := newServer(t, kind, "hss2.example.org", loopback3, nil)

			client := newClient(t)
			setPeers(t, client, primary.peer(kind, 10), secondary.peer(kind, 20))
			waitOpen(t, client, primary.host, secondary.host)

			for range 4 {
				if got := answeredBy(t, send(t, client, routed())); got != primary.host {
					t.Fatalf("answered by %s", got)
				}
			}

			if secondary.calls.Load() != 0 {
				t.Fatalf("the secondary got %d requests", secondary.calls.Load())
			}
		})
	}
}

func TestSendSharesEqualPriorities(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			a := newServer(t, kind, "hss1.example.org", loopback2, nil)
			b := newServer(t, kind, "hss2.example.org", loopback3, nil)

			client := newClient(t)
			setPeers(t, client, a.peer(kind, 10), b.peer(kind, 10))
			waitOpen(t, client, a.host, b.host)

			for range 6 {
				send(t, client, routed())
			}

			if a.calls.Load() != 3 || b.calls.Load() != 3 {
				t.Fatalf("shared %d/%d", a.calls.Load(), b.calls.Load())
			}
		})
	}
}

func TestSendUsesTheSecondaryWhileThePrimaryIsDown(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			secondary := newServer(t, kind, "hss2.example.org", loopback3, nil)

			down := Peer{
				ID: "hss1.example.org", Host: "hss1.example.org", Addresses: []netip.Addr{loopback2}, Transports: []Transport{kind},
				Applications: []Application{sgdApp}, Routes: []Route{{Realm: routeRealm, Application: testAppID, Priority: 1}},
				Dial: &Dial{Port: 1},
			}

			client := newClient(t)
			setPeers(t, client, down, secondary.peer(kind, 2))
			waitOpen(t, client, secondary.host)

			if got := answeredBy(t, send(t, client, routed(), FailFast())); got != secondary.host {
				t.Fatalf("answered by %s", got)
			}
		})
	}
}

func TestSendFailsOverAPendingRequestWithTheTFlag(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			primary := newServer(t, kind, "hss1.example.org", loopback2, func(_ *server, c *Conn, _ *Message) *Message {
				c.abort("test failover")
				return nil
			})
			secondary := newServer(t, kind, "hss2.example.org", loopback3, nil)

			var reroutes []Reroute

			var mu sync.Mutex

			client := newClient(t, func(cfg *Config) {
				cfg.OnReroute = func(r Reroute) {
					mu.Lock()
					defer mu.Unlock()

					reroutes = append(reroutes, r)
				}
			})
			setPeers(t, client, primary.peer(kind, 1), secondary.peer(kind, 2))
			waitOpen(t, client, primary.host, secondary.host)

			if got := answeredBy(t, send(t, client, routed())); got != secondary.host {
				t.Fatalf("answered by %s", got)
			}

			first, second := <-primary.seen, <-secondary.seen
			if first.Flags&FlagRetransmit != 0 || second.Flags&FlagRetransmit == 0 || first.EndToEndID != second.EndToEndID {
				t.Fatalf("flags %#x then %#x, End-to-End %d then %d", first.Flags, second.Flags, first.EndToEndID, second.EndToEndID)
			}

			mu.Lock()
			defer mu.Unlock()

			if len(reroutes) != 1 || reroutes[0].Reason != RerouteFailover || reroutes[0].From != primary.host {
				t.Fatalf("reroutes = %+v", reroutes)
			}
		})
	}
}

func TestSendFailsOverWhenThePrimaryTurnsSuspect(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			ln := listen(t, kind, loopback2)
			t.Cleanup(func() { _ = ln.Close() })

			silent := make(chan *rawPeer, 1)

			go func() {
				tr, err := ln.accept()
				if err != nil {
					return
				}

				p := &rawPeer{t: t, tr: tr, events: make(chan peerEvent, 64)}

				go func() {
					buf := make([]byte, maxMessageSize)

					for {
						n, err := tr.readMessage(buf)
						if err != nil {
							p.events <- peerEvent{err: err}
							return
						}

						m, err := Unmarshal(append([]byte(nil), buf[:n]...))
						p.events <- peerEvent{m: m, err: err}
					}
				}()

				silent <- p
			}()

			secondary := newServer(t, kind, "hss2.example.org", loopback3, nil)

			client := newClient(t, func(cfg *Config) {
				cfg.WatchdogInterval = 100 * time.Millisecond
				cfg.allowFastWatchdog = true
			})

			addr := listenerAddr(ln)
			setPeers(t, client, Peer{
				ID: "hss1.example.org", Host: "hss1.example.org", Addresses: []netip.Addr{addr.Addr()}, Transports: []Transport{kind},
				Applications: []Application{sgdApp}, Routes: []Route{{Realm: routeRealm, Application: testAppID, Priority: 1}},
				Dial: &Dial{Port: addr.Port()},
			}, secondary.peer(kind, 2))

			p := <-silent
			req := p.recv()

			cea := &Message{CommandCode: CommandCapabilitiesExchange, HopByHopID: req.HopByHopID, EndToEndID: req.EndToEndID, AVPs: []AVP{
				Unsigned32(AVPResultCode, AVPFlagMandatory, 0, ResultSuccess),
				UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "hss1.example.org"),
				UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, routeRealm),
				Address(AVPHostIPAddress, AVPFlagMandatory, 0, loopback2),
				Unsigned32(AVPVendorID, AVPFlagMandatory, 0, 0),
				UTF8String(AVPProductName, 0, 0, "silent"),
				appAVP(sgdApp),
			}}
			p.send(cea)
			waitOpen(t, client, "hss1.example.org", secondary.host)

			start := time.Now()

			if got := answeredBy(t, send(t, client, routed())); got != secondary.host || time.Since(start) > 2*time.Second {
				t.Fatalf("answered by %s after %s", got, time.Since(start))
			}

			if got := <-secondary.seen; got.Flags&FlagRetransmit == 0 {
				t.Fatal("the failed-over request has no T flag")
			}
		})
	}
}

func TestSendRetriesAnotherPeerOnTooBusyAndUnableToDeliver(t *testing.T) {
	for _, code := range []uint32{ResultTooBusy, ResultUnableToDeliver} {
		t.Run(ResultName(code), func(t *testing.T) {
			kind := TransportTCP
			refusing := newServer(t, kind, "hss1.example.org", loopback2, func(_ *server, c *Conn, req *Message) *Message {
				return c.Answer(req, code)
			})
			secondary := newServer(t, kind, "hss2.example.org", loopback3, nil)

			client := newClient(t)
			setPeers(t, client, refusing.peer(kind, 1), secondary.peer(kind, 2))
			waitOpen(t, client, refusing.host, secondary.host)

			if got := answeredBy(t, send(t, client, routed())); got != secondary.host {
				t.Fatalf("answered by %s", got)
			}

			first, second := <-refusing.seen, <-secondary.seen
			if first.EndToEndID != second.EndToEndID || first.Flags&FlagRetransmit != 0 || second.Flags&FlagRetransmit == 0 {
				t.Fatalf("End-to-End %d then %d, flags %#x then %#x", first.EndToEndID, second.EndToEndID, first.Flags, second.Flags)
			}
		})
	}
}

func TestSendReturnsTheLastRefusalWhenEveryPeerRefuses(t *testing.T) {
	kind := TransportTCP
	a := newServer(t, kind, "hss1.example.org", loopback2, busy)
	b := newServer(t, kind, "hss2.example.org", loopback3, busy)

	client := newClient(t)
	setPeers(t, client, a.peer(kind, 1), b.peer(kind, 2))
	waitOpen(t, client, a.host, b.host)

	ans := send(t, client, routed())
	if resultCode(t, ans) != ResultTooBusy || answeredBy(t, ans) != b.host || a.calls.Load() != 1 || b.calls.Load() != 1 {
		t.Fatalf("answer %d from %s after %d/%d calls", resultCode(t, ans), answeredBy(t, ans), a.calls.Load(), b.calls.Load())
	}
}

func TestSendToADestinationHostPeerStaysOnIt(t *testing.T) {
	kind := TransportTCP
	bound := newServer(t, kind, "hss1.example.org", loopback2, busy)
	other := newServer(t, kind, "hss2.example.org", loopback3, nil)

	client := newClient(t)
	setPeers(t, client, bound.peer(kind, 1), other.peer(kind, 2))
	waitOpen(t, client, bound.host, other.host)

	ans := send(t, client, routed(UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "HSS1.example.org")))
	if resultCode(t, ans) != ResultTooBusy || other.calls.Load() != 0 {
		t.Fatalf("answer %d, other peer got %d", resultCode(t, ans), other.calls.Load())
	}

	setPeers(t, client, Peer{
		ID: bound.host, Host: bound.host, Addresses: []netip.Addr{loopback2}, Transports: []Transport{kind},
		Applications: []Application{sgdApp}, Dial: &Dial{Port: 1},
	}, other.peer(kind, 2))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := client.Send(ctx, routed(UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, bound.host)), FailFast()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Send to a down Destination-Host = %v", err)
	}
}

func TestSendRoutesAnUnknownDestinationHostByRealm(t *testing.T) {
	kind := TransportTCP
	agent := newServer(t, kind, "dra.example.org", loopback2, nil)

	client := newClient(t)
	setPeers(t, client, agent.peer(kind, 1))
	waitOpen(t, client, agent.host)

	ans := send(t, client, routed(UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "pcrf7.example.org")))
	if resultCode(t, ans) != ResultUnableToDeliver || answeredBy(t, ans) != agent.host {
		t.Fatalf("answer %d from %s", resultCode(t, ans), answeredBy(t, ans))
	}
}

func TestSendWithoutARoute(t *testing.T) {
	client := newClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := client.Send(ctx, routed()); !errors.Is(err, ErrUnableToDeliver) {
		t.Fatalf("Send = %v", err)
	}

	noRealm := request()
	if _, err := client.Send(ctx, noRealm); !errors.Is(err, ErrUnableToDeliver) {
		t.Fatalf("Send without Destination-Realm = %v", err)
	}
}

func TestSendWaitsForARouteUnlessFailFast(t *testing.T) {
	client := newClient(t)
	setPeers(t, client, Peer{
		ID: "hss", Host: "hss1.example.org", Addresses: []netip.Addr{loopback2}, Transports: []Transport{TransportTCP},
		Applications: []Application{sgdApp}, Routes: []Route{{Realm: routeRealm, Application: testAppID}}, Dial: &Dial{Port: 1},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := client.Send(ctx, routed()); !errors.Is(err, ErrNotConnected) || !isTimeout(err) || time.Since(start) < 150*time.Millisecond {
		t.Fatalf("Send = %v after %s", err, time.Since(start))
	}

	start = time.Now()
	if _, err := client.Send(context.Background(), routed(), FailFast()); !errors.Is(err, ErrNotConnected) || time.Since(start) > 100*time.Millisecond {
		t.Fatalf("FailFast Send = %v after %s", err, time.Since(start))
	}
}

func TestRoutesFollowSetPeersWithoutReconnecting(t *testing.T) {
	kind := TransportTCP
	a := newServer(t, kind, "hss1.example.org", loopback2, nil)
	b := newServer(t, kind, "hss2.example.org", loopback3, nil)

	client := newClient(t)
	setPeers(t, client, a.peer(kind, 1), b.peer(kind, 2))
	waitOpen(t, client, a.host, b.host)

	since, _ := client.Peer(a.host)

	setPeers(t, client, a.peer(kind, 3), b.peer(kind, 2))

	if got := answeredBy(t, send(t, client, routed())); got != b.host {
		t.Fatalf("answered by %s", got)
	}

	if now, _ := client.Peer(a.host); now.State != PeerOpen || !now.Since.Equal(since.Since) {
		t.Fatalf("changing a route reconnected the peer: %+v", now)
	}

	routes := client.Routes()
	if len(routes) != 1 || routes[0].Realm != routeRealm || routes[0].Application != testAppID ||
		len(routes[0].Peers) != 2 || routes[0].Peers[0].ID != b.host || routes[0].Peers[1].Priority != 3 || routes[0].Peers[0].State != PeerOpen {
		t.Fatalf("Routes = %+v", routes)
	}
}

func TestSetPeersValidatesRoutes(t *testing.T) {
	client := newClient(t)
	base := Peer{ID: "p", Addresses: []netip.Addr{loopback2}, Transports: []Transport{TransportTCP}, Applications: []Application{sgdApp}}

	for name, routes := range map[string][]Route{
		"invalid realm": {{Realm: "bad realm", Application: testAppID}},
		"empty realm":   {{Application: testAppID}},
		"duplicate":     {{Realm: routeRealm, Application: testAppID}, {Realm: strings.ToUpper(routeRealm), Application: testAppID, Priority: 4}},
		"unadvertised":  {{Realm: routeRealm, Application: otherAppID}},
	} {
		p := base
		p.Routes = routes

		if err := client.SetPeers([]Peer{p}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	relay := base
	relay.Applications = []Application{{ID: RelayApplicationID}}
	relay.Routes = []Route{{Realm: routeRealm, Application: testAppID}}

	if err := client.SetPeers([]Peer{relay}); err != nil {
		t.Fatalf("relay route: %v", err)
	}
}

func TestSendIsDeliveredWithItsAVPsUntouched(t *testing.T) {
	kind := TransportTCP
	s := newServer(t, kind, "hss1.example.org", loopback2, nil)

	client := newClient(t)
	setPeers(t, client, s.peer(kind, 1))
	waitOpen(t, client, s.host)

	req := routed(UTF8String(AVPUserName, AVPFlagMandatory, 0, "alice"))
	before := slices.Clone(req.AVPs)

	send(t, client, req)

	if !slices.EqualFunc(req.AVPs, before, func(a, b AVP) bool { return a.Code == b.Code && string(a.Data) == string(b.Data) }) {
		t.Fatal("Send modified the caller's request")
	}

	got := <-s.seen
	if !got.IsRequest() || got.Flags&FlagRetransmit != 0 {
		t.Fatalf("flags %#x", got.Flags)
	}
}
