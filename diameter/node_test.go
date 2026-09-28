// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/sctp"
)

func pair(t *testing.T, kind Transport, opts ...func(*Config)) (*Node, *Node, netip.AddrPort) {
	t.Helper()

	hssCfg := testConfig("hss.example.org")
	for _, opt := range opts {
		opt(&hssCfg)
	}

	hss := newTestNode(t, hssCfg)

	smscCfg := testConfig("smsc.example.org")
	smscCfg.Identity.HostIPAddresses = []netip.Addr{loopback2}

	for _, opt := range opts {
		opt(&smscCfg)
	}

	smsc := newTestNode(t, smscCfg)

	if err := hss.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transport: kind, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
		t.Fatal(err)
	}

	return hss, smsc, serveOn(t, hss, kind, loopback1)
}

func TestNodesExchangeRequestsBothWays(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			hss, smsc, addr := pair(t, kind)

			if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: kind, Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			eventually(t, "the SMSC to reach the HSS", func() bool { return doOK(smsc, "hss") })
			eventually(t, "the HSS to reach the SMSC", func() bool { return doOK(hss, "smsc") })

			s, _ := smsc.Peer("hss")
			if s.Host != "hss.example.org" || s.Realm != "example.org" || s.State != PeerOpen || len(s.Applications) != 1 {
				t.Fatalf("SMSC view = %+v", s)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if ans, err := smsc.DoHost(ctx, "HSS.example.org", request()); err != nil || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("DoHost = %v", err)
			}

			other := request()
			other.ApplicationID = otherAppID

			if _, err := smsc.Do(ctx, "hss", other); !errors.Is(err, ErrApplicationUnsupported) {
				t.Fatalf("unnegotiated application = %v", err)
			}
		})
	}
}

func TestOutboundConnectionBindsHostIPAddresses(t *testing.T) {
	var remote atomic.Value

	hss, smsc, addr := pair(t, TransportSCTP, func(cfg *Config) {
		cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
			remote.Store(c.RemoteAddr())
			return c.Answer(req, ResultSuccess)
		})
	})
	_ = hss

	if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the SMSC to reach the HSS", func() bool { return doOK(smsc, "hss") })

	if got := remote.Load(); got != loopback2 {
		t.Fatalf("HSS saw the SMSC at %v", got)
	}
}

func TestSetPeersReconciles(t *testing.T) {
	hss, smsc, addr := pair(t, TransportSCTP)
	hssPeer := Peer{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: TransportSCTP, Applications: []Application{sgdApp}}

	if err := smsc.SetPeers([]Peer{hssPeer}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the peer to open", func() bool { return doOK(smsc, "hss") })

	before, _ := smsc.Peer("hss")

	if err := smsc.SetPeers([]Peer{hssPeer}); err != nil {
		t.Fatal(err)
	}

	if after, _ := smsc.Peer("hss"); after.State != PeerOpen || !after.Since.Equal(before.Since) {
		t.Fatalf("unchanged peer was disturbed: %+v", after)
	}

	if err := smsc.SetPeers(nil); err != nil {
		t.Fatal(err)
	}

	if _, ok := smsc.Peer("hss"); ok {
		t.Fatal("removed peer still listed")
	}

	if _, err := smsc.Do(context.Background(), "hss", request()); !errors.Is(err, ErrUnknownPeer) {
		t.Fatalf("Do after removal = %v", err)
	}

	eventually(t, "the HSS to see the SMSC leave", func() bool {
		s, _ := hss.Peer("smsc")
		return s.State == PeerDown
	})
}

func TestSetPeersValidation(t *testing.T) {
	n := newTestNode(t, testConfig("ella.example.org"))
	base := Peer{ID: "a", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}

	with := func(f func(*Peer)) Peer {
		p := base
		f(&p)

		return p
	}

	tests := map[string][]Peer{
		"no ID":                {with(func(p *Peer) { p.ID = "" })},
		"no address":           {with(func(p *Peer) { p.Addresses = nil })},
		"unspecified address":  {with(func(p *Peer) { p.Addresses = []netip.Addr{netip.IPv4Unspecified()} })},
		"no application":       {with(func(p *Peer) { p.Applications = nil })},
		"unknown transport":    {with(func(p *Peer) { p.Transport = 7 })},
		"unknown transport 1":  {with(func(p *Peer) { p.Transport = 1 })},
		"duplicate ID":         {base, base},
		"shared host":          {with(func(p *Peer) { p.Host = "x" }), with(func(p *Peer) { p.ID = "b"; p.Host = "X" })},
		"shared hostless addr": {base, with(func(p *Peer) { p.ID = "b" })},
	}

	for name, peers := range tests {
		if err := n.SetPeers(peers); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	if err := n.SetPeers([]Peer{base, with(func(p *Peer) { p.ID = "b"; p.Host = "b.example.org" })}); err != nil {
		t.Errorf("same address with distinct hosts: %v", err)
	}
}

func TestElectionLeavesOneConnection(t *testing.T) {
	for _, hostless := range []bool{false, true} {
		t.Run(map[bool]string{false: "hosts", true: "hostless"}[hostless], func(t *testing.T) {
			a := newTestNode(t, testConfig("a.example.org"))

			bCfg := testConfig("b.example.org")
			bCfg.Identity.HostIPAddresses = []netip.Addr{loopback2}
			b := newTestNode(t, bCfg)

			aAddr := serveOn(t, a, TransportSCTP, loopback1)
			bAddr := serveOn(t, b, TransportSCTP, loopback2)

			host := func(h string) string {
				if hostless {
					return ""
				}

				return h
			}

			if err := a.SetPeers([]Peer{{ID: "b", Host: host("b.example.org"), Addresses: []netip.Addr{loopback2}, Port: bAddr.Port(), Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			if err := b.SetPeers([]Peer{{ID: "a", Host: host("a.example.org"), Addresses: []netip.Addr{loopback1}, Port: aAddr.Port(), Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			eventually(t, "both sides to open", func() bool { return doOK(a, "b") && doOK(b, "a") })

			time.Sleep(300 * time.Millisecond)

			if !doOK(a, "b") || !doOK(b, "a") {
				t.Fatal("connection lost after the election")
			}

			for _, n := range []*Node{a, b} {
				if conns := n.allConns(); len(conns) != 1 {
					t.Fatalf("%s keeps %d connections", n.cfg.Identity.OriginHost, len(conns))
				}
			}
		})
	}
}

func TestReconnectAfterPeerRestartUsesReopen(t *testing.T) {
	hssCfg := testConfig("hss.example.org")
	hssCfg.WatchdogInterval = 100 * time.Millisecond
	hssCfg.allowFastWatchdog = true

	hss := newTestNode(t, hssCfg)

	if err := hss.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transport: TransportSCTP, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
		t.Fatal(err)
	}

	addr := serveOn(t, hss, TransportSCTP, loopback1)

	smscCfg := testConfig("smsc.example.org")
	smscCfg.Identity.HostIPAddresses = []netip.Addr{loopback2}
	smscCfg.WatchdogInterval = 100 * time.Millisecond
	smscCfg.allowFastWatchdog = true

	var states []PeerState

	var mu sync.Mutex

	smscCfg.OnPeerStateChange = func(s PeerStatus) {
		mu.Lock()

		states = append(states, s.State)
		mu.Unlock()
	}

	smsc := newTestNode(t, smscCfg)

	if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: TransportSCTP, Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the first connection", func() bool { return doOK(smsc, "hss") })

	for _, c := range hss.allConns() {
		c.abort("test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	if ans, err := smsc.Do(ctx, "hss", request()); err != nil || resultCode(t, ans) != ResultSuccess {
		t.Fatalf("Do across the reconnect = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	joined := make([]string, len(states))
	for i, s := range states {
		joined[i] = s.String()
	}

	if got := strings.Join(joined, ","); !strings.Contains(got, "open,down") || !strings.Contains(got, "reopen,open") {
		t.Fatalf("state changes = %s", got)
	}
}

func TestDoNotWantToTalkToYouSuppressesRedial(t *testing.T) {
	hss, smsc, addr := pair(t, TransportSCTP)

	if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: TransportSCTP, Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the connection", func() bool { return doOK(smsc, "hss") })

	if err := hss.SetPeers(nil); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the SMSC to see the DPR", func() bool {
		s, _ := smsc.Peer("hss")
		return s.State == PeerDown
	})

	time.Sleep(500 * time.Millisecond)

	smsc.mu.Lock()
	p := smsc.byID["hss"]
	suppressed, idle := p.suppress, p.initiator == nil && p.open == nil
	smsc.mu.Unlock()

	if !suppressed || !idle {
		t.Fatalf("suppress=%v idle=%v", suppressed, idle)
	}
}

func TestFailoverRetransmitsWithTFlag(t *testing.T) {
	var calls atomic.Int32

	hss, smsc, addr := pair(t, TransportSCTP, func(cfg *Config) {
		cfg.WatchdogInterval = 100 * time.Millisecond
		cfg.allowFastWatchdog = true
		cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
			if calls.Add(1) == 1 {
				c.abort("test failover")
			}

			return c.Answer(req, ResultSuccess)
		})
	})
	_ = hss

	if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: TransportSCTP, Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the connection", func() bool {
		s, _ := smsc.Peer("hss")
		return s.State == PeerOpen
	})

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	if ans, err := smsc.Do(ctx, "hss", request()); err != nil || resultCode(t, ans) != ResultSuccess {
		t.Fatalf("Do = %v", err)
	}

	if calls.Load() != 1 {
		t.Fatalf("handler ran %d times; the retransmission should have hit the duplicate cache", calls.Load())
	}
}

func TestDoWaitsBoundedByContext(t *testing.T) {
	n := newTestNode(t, testConfig("smsc.example.org"))

	if err := n.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{loopback1}, Port: 1, Transport: TransportSCTP, Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()

	_, err := n.Do(ctx, "hss", request())
	if !errors.Is(err, ErrNotConnected) || !isTimeout(err) || time.Since(start) < 150*time.Millisecond {
		t.Fatalf("Do = %v after %s", err, time.Since(start))
	}

	if s, _ := n.Peer("hss"); s.LastError == "" {
		t.Fatal("dial failure not reported")
	}

	if _, err := n.Do(ctx, "nope", request()); !errors.Is(err, ErrUnknownPeer) {
		t.Fatalf("unknown peer = %v", err)
	}
}

func TestReconnectBackoff(t *testing.T) {
	requireSCTP(t)

	var lc sctp.ListenConfig

	tl, err := lc.Listen(context.Background(), &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: loopback1.AsSlice()}}})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = tl.Close() }()

	var accepted atomic.Int32

	go func() {
		for {
			c, err := tl.Accept()
			if err != nil {
				return
			}

			accepted.Add(1)

			_ = c.Close()
		}
	}()

	cfg := testConfig("smsc.example.org")
	cfg.ReconnectInterval = 100 * time.Millisecond
	cfg.MaxReconnectInterval = time.Second

	n := newTestNode(t, cfg)
	addr := netip.AddrPortFrom(loopback1, uint16(tl.Addr().(*sctp.SCTPAddr).Port))

	if err := n.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: TransportSCTP, Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1500 * time.Millisecond)

	if got := accepted.Load(); got < 2 || got > 6 {
		t.Fatalf("%d connection attempts in 1.5s", got)
	}
}

func TestWatchdogClosesSilentPeer(t *testing.T) {
	_, p := ellaWithSMSC(t, TransportSCTP, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}, func(cfg *Config) {
		cfg.WatchdogInterval = 100 * time.Millisecond
		cfg.allowFastWatchdog = true
	})

	openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

	if dwr := p.recv(); dwr.CommandCode != CommandDeviceWatchdog || !dwr.IsRequest() {
		t.Fatalf("expected a DWR, got %+v", dwr)
	}

	p.expectClosed()
}

func TestShutdown(t *testing.T) {
	hss, smsc, addr := pair(t, TransportSCTP)

	if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: TransportSCTP, Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the connection", func() bool { return doOK(smsc, "hss") })

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	if err := hss.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown = %v", err)
	}

	eventually(t, "the SMSC to see the DPR", func() bool {
		s, _ := smsc.Peer("hss")
		return s.State != PeerOpen
	})

	if err := hss.SetPeers(nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("SetPeers after Shutdown = %v", err)
	}

	if err := hss.Serve(NewSCTPListener(nil, nil)); !errors.Is(err, ErrClosed) {
		t.Fatalf("Serve after Shutdown = %v", err)
	}

	if err := hss.Shutdown(ctx); !errors.Is(err, ErrClosed) {
		t.Fatalf("second Shutdown = %v", err)
	}
}

func TestShutdownWaitsForInFlightRequests(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	hss, smsc, addr := pair(t, TransportSCTP, func(cfg *Config) {
		cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
			if req.CommandCode == 8388647 {
				close(started)
				<-release
			}

			return c.Answer(req, ResultSuccess)
		})
	})

	if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: TransportSCTP, Applications: []Application{sgdApp}}}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the connection", func() bool {
		s, _ := smsc.Peer("hss")
		return s.State == PeerOpen
	})

	answered := make(chan error, 1)

	go func() {
		ans, err := smsc.Do(context.Background(), "hss", request())
		if err == nil && resultCode(t, ans) != ResultSuccess {
			err = errors.New("unexpected result")
		}

		answered <- err
	}()

	<-started

	shutdown := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		shutdown <- hss.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	select {
	case <-shutdown:
		t.Fatal("Shutdown returned with a request in flight")
	default:
	}

	close(release)

	if err := <-answered; err != nil {
		t.Fatalf("in-flight request = %v", err)
	}

	if err := <-shutdown; err != nil {
		t.Fatalf("Shutdown = %v", err)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	valid := testConfig("a.example.org")

	tests := map[string]func(*Config){
		"no host":            func(c *Config) { c.Identity.OriginHost = "" },
		"no realm":           func(c *Config) { c.Identity.OriginRealm = "" },
		"no address":         func(c *Config) { c.Identity.HostIPAddresses = nil },
		"unspecified":        func(c *Config) { c.Identity.HostIPAddresses = []netip.Addr{netip.IPv4Unspecified()} },
		"no product":         func(c *Config) { c.Identity.ProductName = "" },
		"no handler":         func(c *Config) { c.Handler = nil },
		"unknown, no apps":   func(c *Config) { c.AcceptUnknownPeers = true },
		"negative timeout":   func(c *Config) { c.RequestTimeout = -1 },
		"fast watchdog":      func(c *Config) { c.WatchdogInterval = time.Second },
		"negative the limit": func(c *Config) { c.MaxConcurrentRequests = -1 },
	}

	for name, mutate := range tests {
		cfg := valid
		mutate(&cfg)

		if _, err := New(cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	n, err := New(valid)
	if err != nil {
		t.Fatal(err)
	}

	if n.cfg.WatchdogInterval != DefaultWatchdogInterval || n.cfg.OriginStateID == 0 || n.cfg.MaxDuplicateEntries != DefaultMaxDuplicateEntries {
		t.Fatalf("defaults = %+v", n.cfg)
	}

	if id := n.NewSessionID(); !strings.HasPrefix(id, "a.example.org;") {
		t.Fatalf("Session-Id = %s", id)
	}
}
