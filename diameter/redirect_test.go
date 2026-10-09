// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type staticResolver map[string]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	a, ok := r[strings.ToLower(host)]
	if !ok {
		return nil, errors.New("no such host")
	}

	return []netip.Addr{a}, nil
}

func redirectingTo(target *server, kind Transport, usage RedirectHostUsage, ttl time.Duration) func(*server, *Conn, *Message) *Message {
	return func(s *server, c *Conn, req *Message) *Message {
		ans, err := NewRedirectAnswer(req, s.node.Identity(), Redirect{
			Hosts:        []URI{{Host: target.host, Port: target.addr.Port(), Transport: kind}},
			Usage:        usage,
			MaxCacheTime: ttl,
		})
		if err != nil {
			panic(err)
		}

		return ans
	}
}

func redirectClient(t *testing.T, target *server, opts ...func(*Config)) *Node {
	t.Helper()

	return newClient(t, append([]func(*Config){func(cfg *Config) {
		cfg.Resolver = staticResolver{target.host: target.addr.Addr()}
	}}, opts...)...)
}

func dynamicPeers(n *Node) []PeerStatus {
	var out []PeerStatus

	for _, p := range n.Peers() {
		if p.Dynamic {
			out = append(out, p)
		}
	}

	return out
}

func TestSendFollowsARedirectToADynamicPeer(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			home := newServer(t, kind, "hss7.example.org", loopback3, nil)
			slf := newServer(t, kind, "slf.example.org", loopback2, redirectingTo(home, kind, DontCache, 0))

			var reasons []RerouteReason

			client := redirectClient(t, home, func(cfg *Config) {
				cfg.OnReroute = func(r Reroute) { reasons = append(reasons, r.Reason) }
			})
			setPeers(t, client, slf.peer(kind, 1))
			waitOpen(t, client, slf.host)

			ans := send(t, client, routed(), FailFast())
			if answeredBy(t, ans) != home.host || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("answer %d from %s", resultCode(t, ans), answeredBy(t, ans))
			}

			first, redirected := <-slf.seen, <-home.seen
			if host, _ := redirected.Find(AVPDestinationHost, 0); host.UTF8String() != home.host {
				t.Fatalf("the redirected request has Destination-Host %q", host.UTF8String())
			}

			if first.EndToEndID != redirected.EndToEndID || redirected.Flags&FlagRetransmit == 0 {
				t.Fatal("the redirected request is not the original with the T flag")
			}

			if len(reasons) != 1 || reasons[0] != RerouteRedirect {
				t.Fatalf("reroutes = %v", reasons)
			}

			eventually(t, "the dynamic peer to be released", func() bool { return len(dynamicPeers(client)) == 0 })

			send(t, client, routed())

			if slf.calls.Load() != 2 {
				t.Fatalf("DONT_CACHE redirect was cached: the SLF got %d requests", slf.calls.Load())
			}
		})
	}
}

func TestSendCachesRedirects(t *testing.T) {
	kind := TransportTCP

	for _, tc := range []struct {
		usage   RedirectHostUsage
		cached  func() *Message
		missing func() *Message
	}{
		{
			usage:   AllSession,
			cached:  func() *Message { return routed() },
			missing: func() *Message { return otherSession(routed()) },
		},
		{
			usage:   AllUser,
			cached:  func() *Message { return otherSession(routed(user("alice"))) },
			missing: func() *Message { return routed(user("bob")) },
		},
		{
			usage:  RealmAndApplication,
			cached: func() *Message { return otherSession(routed()) },
		},
		{
			usage:  AllRealm,
			cached: func() *Message { return otherSession(routed()) },
		},
		{
			usage:  AllApplication,
			cached: func() *Message { return otherSession(routed()) },
		},
		{
			usage:  AllHost,
			cached: func() *Message { return otherSession(routed()) },
		},
	} {
		t.Run(tc.usage.String(), func(t *testing.T) {
			home := newServer(t, kind, "hss7.example.org", loopback3, nil)
			agent := newServer(t, kind, "dra.example.org", loopback2, redirectingTo(home, kind, tc.usage, time.Hour))

			client := redirectClient(t, home)
			setPeers(t, client, agent.peer(kind, 1))
			waitOpen(t, client, agent.host)

			send(t, client, routed(user("alice")))

			if got := answeredBy(t, send(t, client, tc.cached())); got != home.host || agent.calls.Load() != 1 {
				t.Fatalf("cached request answered by %s after %d agent requests", got, agent.calls.Load())
			}

			if tc.missing != nil {
				send(t, client, tc.missing())

				if agent.calls.Load() != 2 {
					t.Fatalf("a request outside the %s entry skipped the agent", tc.usage)
				}
			}

			if len(dynamicPeers(client)) != 1 {
				t.Fatalf("dynamic peers = %+v", dynamicPeers(client))
			}
		})
	}
}

func TestRedirectCacheExpires(t *testing.T) {
	kind := TransportTCP
	home := newServer(t, kind, "hss7.example.org", loopback3, nil)
	agent := newServer(t, kind, "dra.example.org", loopback2, redirectingTo(home, kind, AllRealm, time.Second))

	client := redirectClient(t, home)
	setPeers(t, client, agent.peer(kind, 1))
	waitOpen(t, client, agent.host)

	send(t, client, routed())
	send(t, client, routed())

	if agent.calls.Load() != 1 {
		t.Fatalf("the agent got %d requests before expiry", agent.calls.Load())
	}

	eventually(t, "the cached route and its peer to expire", func() bool { return len(dynamicPeers(client)) == 0 })

	send(t, client, routed())

	if agent.calls.Load() != 2 {
		t.Fatalf("the agent got %d requests after expiry", agent.calls.Load())
	}
}

func TestRedirectCacheDroppedWhenTheHostIsLost(t *testing.T) {
	kind := TransportTCP
	home := newServer(t, kind, "hss7.example.org", loopback3, nil)
	agent := newServer(t, kind, "dra.example.org", loopback2, redirectingTo(home, kind, AllRealm, time.Hour))

	client := redirectClient(t, home)
	setPeers(t, client, agent.peer(kind, 1))
	waitOpen(t, client, agent.host)

	send(t, client, routed())

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	if err := home.node.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the lost host to be forgotten", func() bool { return len(dynamicPeers(client)) == 0 })

	client.mu.Lock()
	entries := len(client.cache)
	client.mu.Unlock()

	if entries != 0 {
		t.Fatalf("%d cached routes survive the lost host", entries)
	}
}

func TestSendRedirectsToAConfiguredPeer(t *testing.T) {
	kind := TransportTCP
	home := newServer(t, kind, "hss7.example.org", loopback3, nil)
	agent := newServer(t, kind, "dra.example.org", loopback2, redirectingTo(home, kind, AllSession, time.Hour))

	direct := home.peer(kind, 0)
	direct.Routes = nil

	client := newClient(t, func(cfg *Config) { cfg.Resolver = staticResolver{} })
	setPeers(t, client, agent.peer(kind, 1), direct)
	waitOpen(t, client, agent.host, home.host)

	if got := answeredBy(t, send(t, client, routed())); got != home.host {
		t.Fatalf("answered by %s", got)
	}

	if len(dynamicPeers(client)) != 0 {
		t.Fatal("a dynamic peer was created for a configured host")
	}
}

func TestSendFollowsOneRedirectOnly(t *testing.T) {
	kind := TransportTCP

	var loop *server

	loop = newServer(t, kind, "hss7.example.org", loopback3, func(s *server, c *Conn, req *Message) *Message {
		return redirectingTo(loop, kind, DontCache, 0)(s, c, req)
	})
	agent := newServer(t, kind, "dra.example.org", loopback2, redirectingTo(loop, kind, DontCache, 0))

	client := redirectClient(t, loop)
	setPeers(t, client, agent.peer(kind, 1))
	waitOpen(t, client, agent.host)

	ans := send(t, client, routed())
	if resultCode(t, ans) != ResultRedirectIndication || agent.calls.Load() != 1 || loop.calls.Load() != 1 {
		t.Fatalf("answer %d after %d/%d requests", resultCode(t, ans), agent.calls.Load(), loop.calls.Load())
	}
}

func TestSendDoesNotFollowAnUnusableRedirect(t *testing.T) {
	kind := TransportTCP
	home := newServer(t, kind, "hss7.example.org", loopback3, nil)
	agent := newServer(t, kind, "dra.example.org", loopback2, func(s *server, c *Conn, req *Message) *Message {
		ans, _ := NewRedirectAnswer(req, s.node.Identity(), Redirect{Hosts: []URI{
			{Host: "secure.example.org", Port: DefaultSecurePort, Transport: TransportTCP, Secure: true},
			{Host: "unknown.example.org", Port: DefaultPort, Transport: TransportTCP},
		}})

		return ans
	})

	client := redirectClient(t, home)
	setPeers(t, client, agent.peer(kind, 1))
	waitOpen(t, client, agent.host)

	if ans := send(t, client, routed()); resultCode(t, ans) != ResultRedirectIndication {
		t.Fatalf("answer %d", resultCode(t, ans))
	}
}

func TestRedirectAnswerRoundTrip(t *testing.T) {
	req := routed()
	id := Identity{OriginHost: "dra.example.org", OriginRealm: routeRealm}

	r := Redirect{
		Hosts:        []URI{{Host: "pcrf1.example.org", Port: 3868, Transport: TransportSCTP}, {Host: "pcrf2.example.org", Port: 3869, Transport: TransportTCP}},
		Usage:        AllSession,
		MaxCacheTime: 90 * time.Second,
	}

	ans, err := NewRedirectAnswer(req, id, r)
	if err != nil {
		t.Fatal(err)
	}

	if ans.Flags&FlagError == 0 || resultCode(t, ans) != ResultRedirectIndication {
		t.Fatalf("flags %#x, result %d", ans.Flags, resultCode(t, ans))
	}

	b, err := ans.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseRedirect(decoded)
	if err != nil || len(got.Hosts) != 2 || got.Hosts[0] != r.Hosts[0] || got.Hosts[1] != r.Hosts[1] || got.Usage != r.Usage || got.MaxCacheTime != r.MaxCacheTime {
		t.Fatalf("ParseRedirect = %+v, %v", got, err)
	}

	for name, bad := range map[string]Redirect{
		"no host":          {},
		"usage":            {Hosts: r.Hosts, Usage: 7},
		"no cache time":    {Hosts: r.Hosts, Usage: AllRealm},
		"subsecond":        {Hosts: r.Hosts, Usage: AllRealm, MaxCacheTime: 1500 * time.Millisecond},
		"invalid hostname": {Hosts: []URI{{Host: "bad host", Port: 1, Transport: TransportTCP}}},
	} {
		if _, err := NewRedirectAnswer(req, id, bad); !errors.Is(err, ErrInvalidRedirect) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseRedirectRejects(t *testing.T) {
	id := Identity{OriginHost: "dra.example.org", OriginRealm: routeRealm}
	base := NewAnswer(routed(), id, ResultRedirectIndication)

	with := func(avps ...AVP) *Message {
		m := *base
		m.AVPs = append(append([]AVP(nil), base.AVPs...), avps...)

		return &m
	}

	host := UTF8String(AVPRedirectHost, AVPFlagMandatory, 0, "aaa://pcrf1.example.org")

	for name, ans := range map[string]*Message{
		"not a redirect":         NewAnswer(routed(), id, ResultTooBusy),
		"no Redirect-Host":       with(),
		"bad URI":                with(UTF8String(AVPRedirectHost, AVPFlagMandatory, 0, "http://pcrf1.example.org")),
		"usage without max time": with(host, Unsigned32(AVPRedirectHostUsage, AVPFlagMandatory, 0, uint32(AllRealm))),
		"unknown usage":          with(host, Unsigned32(AVPRedirectHostUsage, AVPFlagMandatory, 0, 9)),
	} {
		if _, err := ParseRedirect(ans); !errors.Is(err, ErrInvalidRedirect) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	got, err := ParseRedirect(with(host))
	if err != nil || got.Usage != DontCache || got.Hosts[0] != (URI{Host: "pcrf1.example.org", Port: DefaultPort, Transport: TransportTCP}) {
		t.Fatalf("defaults = %+v, %v", got, err)
	}
}

func TestParseURI(t *testing.T) {
	for in, want := range map[string]URI{
		"aaa://host.example.com":                                      {Host: "host.example.com", Port: 3868, Transport: TransportTCP},
		"aaa://host.example.com;transport=tcp":                        {Host: "host.example.com", Port: 3868, Transport: TransportTCP},
		"aaa://host.example.com:6666;transport=sctp":                  {Host: "host.example.com", Port: 6666, Transport: TransportSCTP},
		"aaa://host.example.com;protocol=diameter":                    {Host: "host.example.com", Port: 3868, Transport: TransportTCP},
		"aaa://host.example.com:6666;transport=tcp;protocol=diameter": {Host: "host.example.com", Port: 6666, Transport: TransportTCP},
		"aaas://host.example.com":                                     {Host: "host.example.com", Port: 5658, Transport: TransportTCP, Secure: true},
		"AAA://host.example.com;Transport=SCTP;Protocol=Diameter":     {Host: "host.example.com", Port: 3868, Transport: TransportSCTP},
		"Aaas://host.example.com:6666":                                {Host: "host.example.com", Port: 6666, Transport: TransportTCP, Secure: true},
	} {
		got, err := ParseURI(in)
		if err != nil || got != want {
			t.Errorf("ParseURI(%q) = %+v, %v", in, got, err)
		}

		if back, err := ParseURI(got.String()); err != nil || back != got {
			t.Errorf("%q does not round-trip: %q", in, got.String())
		}
	}

	for _, in := range []string{
		"", "host.example.com", "http://host.example.com", "aaa://", "aaa:/host.example.com", "aaa://bad_host", "aaa://host.example.com:0",
		"aaa://host.example.com:70000", "aaa://host.example.com;transport=udp", "aaa://host.example.com;protocol=radius",
		"aaa://host.example.com;transport=tcp;transport=sctp", "aaa://host.example.com;foo=bar", "aaa://host.example.com;",
	} {
		if _, err := ParseURI(in); !errors.Is(err, ErrInvalidURI) {
			t.Errorf("ParseURI(%q) = %v", in, err)
		}
	}
}

func user(name string) AVP {
	return UTF8String(AVPUserName, AVPFlagMandatory, 0, name)
}

func otherSession(m *Message) *Message {
	for i, a := range m.AVPs {
		if a.Code == AVPSessionID {
			m.AVPs[i] = UTF8String(AVPSessionID, AVPFlagMandatory, 0, "ims.example.org;1;2")
		}
	}

	return m
}

func TestSendDoesNotLoopOnABusyAllHostTarget(t *testing.T) {
	kind := TransportTCP

	var busyHome atomic.Bool

	home := newServer(t, kind, "hss7.example.org", loopback3, func(_ *server, c *Conn, req *Message) *Message {
		if busyHome.Load() {
			return c.Answer(req, ResultTooBusy)
		}

		return c.Answer(req, ResultSuccess)
	})
	agent := newServer(t, kind, "dra.example.org", loopback2, redirectingTo(home, kind, AllHost, time.Hour))

	client := redirectClient(t, home)
	setPeers(t, client, agent.peer(kind, 1))
	waitOpen(t, client, agent.host)

	send(t, client, routed())
	busyHome.Store(true)

	ans := send(t, client, routed())
	if resultCode(t, ans) != ResultTooBusy || home.calls.Load() != 2 {
		t.Fatalf("answer %d after %d requests to the redirect target", resultCode(t, ans), home.calls.Load())
	}
}
