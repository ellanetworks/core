// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/sh"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
)

const (
	smscHost    = "smsc.node.epc.mnc001.mcc001.3gppnetwork.org"
	smscRealm   = "epc.mnc001.mcc001.3gppnetwork.org"
	ellaHost    = "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org"
	ellaRealm   = "epc.mnc001.mcc001.3gppnetwork.org"
	waitTimeout = 15 * time.Second
)

var loopback = netip.MustParseAddr("127.0.0.1")

func requireSCTP(t *testing.T) {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_SCTP)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("SCTP not available in CI: %v", err)
		}

		t.Skipf("SCTP not available: %v", err)
	}

	_ = syscall.Close(fd)
}

type fakeSMSC struct {
	node *diameter.Node
	ln   *sctp.Listener
	addr netip.AddrPort

	mu        sync.Mutex
	lastError string
}

func requireBindToDevice(t *testing.T) {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = syscall.Close(fd) }()

	if err := syscall.SetsockoptString(fd, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, "lo"); err != nil {
		t.Skipf("SO_BINDTODEVICE not permitted: %v", err)
	}
}

func startFakeSMSC(t *testing.T, port int) *fakeSMSC {
	t.Helper()

	s := &fakeSMSC{}

	node, err := diameter.New(diameter.Config{
		Identity: diameter.Identity{
			OriginHost:      smscHost,
			OriginRealm:     smscRealm,
			HostIPAddresses: []netip.Addr{loopback},
			ProductName:     "fake-smsc",
		},
		Handler: diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
			return c.Answer(req, diameter.ResultSuccess)
		}),
		AcceptUnknownPeers: true,
		UnknownPeerApplications: []diameter.Application{
			{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
			{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
		},
		OnPeerStateChange: func(p diameter.PeerStatus) {
			s.mu.Lock()
			defer s.mu.Unlock()

			if p.LastError != "" {
				s.lastError = p.LastError
			}
		},
	})
	if err != nil {
		t.Fatalf("new fake SMSC: %v", err)
	}

	var lc sctp.ListenConfig

	ln, err := lc.Listen(context.Background(), &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: loopback.AsSlice()}}, Port: port})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	a, ok := ln.Addr().(*sctp.SCTPAddr)
	if !ok {
		t.Fatalf("unexpected listener address %v", ln.Addr())
	}

	go func() { _ = node.Serve(diameter.NewSCTPListener(ln, nil)) }()

	s.node, s.ln, s.addr = node, ln, netip.AddrPortFrom(loopback, uint16(a.Port))

	t.Cleanup(s.stop)

	return s
}

func (s *fakeSMSC) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.node.Shutdown(ctx)
	_ = s.ln.Close()
}

func (s *fakeSMSC) peerError() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.lastError
}

func (s *fakeSMSC) connectedHost() string {
	for _, p := range s.node.Peers() {
		if p.State == diameter.PeerOpen {
			return p.Host
		}
	}

	return ""
}

func (s *fakeSMSC) connectedAddr() netip.Addr {
	for _, p := range s.node.Peers() {
		if p.State == diameter.PeerOpen {
			return p.RemoteAddr
		}
	}

	return netip.Addr{}
}

type settingsSource struct {
	mu       sync.Mutex
	smsc     netip.AddrPort
	smscHost string
	node     diameternode.NodeSettings
	nodeErr  error
}

func newSettingsSource() *settingsSource {
	return &settingsSource{node: diameternode.NodeSettings{MCC: "001", MNC: "01", MMEGroupID: 0x8204, MMECode: 0x01}}
}

func (s *settingsSource) setSMSC(smsc netip.AddrPort) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.smsc = smsc
}

func (s *settingsSource) setNode(node diameternode.NodeSettings, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.node, s.nodeErr = node, err
}

func (s *settingsSource) getPeers(context.Context) ([]diameternode.PeerConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.smsc.IsValid() {
		return nil, nil
	}

	return []diameternode.PeerConfig{{
		ID:      "smsc-1",
		Role:    "smsc",
		Host:    s.smscHost,
		Address: s.smsc.Addr().String(),
		Port:    s.smsc.Port(),
		Applications: []diameter.Application{
			{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
			{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
		},
	}}, nil
}

func (s *settingsSource) getNode(context.Context) (diameternode.NodeSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.node, s.nodeErr
}

func startManager(t *testing.T, source *settingsSource) (*diameternode.Manager, chan struct{}) {
	t.Helper()

	return startListeningManager(t, source, diameternode.ListenConfig{})
}

func startListeningManager(t *testing.T, source *settingsSource, listen diameternode.ListenConfig) (*diameternode.Manager, chan struct{}) {
	t.Helper()

	return runManager(t, diameternode.New(source.getNode, source.getPeers, listen, zap.NewNop()))
}

func runManager(t *testing.T, link *diameternode.Manager) (*diameternode.Manager, chan struct{}) {
	t.Helper()

	link.Handle(s6c.ApplicationID, s6c.CommandSendRoutingInfoForSM, diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return tgpp.NewAnswer(req, c.LocalIdentity(), diameter.ResultUnableToComply)
	}))

	wakeup := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		link.Run(ctx, wakeup)
		close(done)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	return link, wakeup
}

func poke(wakeup chan struct{}) {
	select {
	case wakeup <- struct{}{}:
	default:
	}
}

func smscPeer(link *diameternode.Manager) (diameternode.PeerStatus, bool) {
	for _, p := range link.Peers() {
		if p.Role == "smsc" {
			return p, true
		}
	}

	return diameternode.PeerStatus{}, false
}

func smscState(link *diameternode.Manager) diameter.PeerState {
	p, ok := smscPeer(link)
	if !ok {
		return -1
	}

	return p.State
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(waitTimeout)

	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func TestNodeDisabledWithoutSMSC(t *testing.T) {
	source := newSettingsSource()
	link, _ := startManager(t, source)

	time.Sleep(100 * time.Millisecond)

	if peers := link.Peers(); len(peers) != 0 {
		t.Fatalf("peers reported although SMS is disabled: %+v", peers)
	}

	if link.Node() != nil {
		t.Fatal("a Diameter node was created although SMS is disabled")
	}
}

func TestNodeConnectsToSMSC(t *testing.T) {
	requireSCTP(t)

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link, _ := startManager(t, source)

	waitFor(t, "link up", func() bool { return smscState(link) == diameter.PeerOpen })

	identity, err := link.Identity(context.Background())
	if err != nil {
		t.Fatalf("Identity: %v", err)
	}

	if identity.Host != ellaHost || identity.Realm != ellaRealm {
		t.Fatalf("identity = %+v, want %s / %s", identity, ellaHost, ellaRealm)
	}

	peer, _ := smscPeer(link)
	if peer.Host != smscHost || peer.Realm != smscRealm || peer.Address != smsc.addr.Addr().String() || peer.Port != smsc.addr.Port() || peer.Since.IsZero() {
		t.Fatalf("peer = %+v, want %s / %s at %s", peer, smscHost, smscRealm, smsc.addr)
	}

	waitFor(t, "SMSC to see Ella", func() bool { return smsc.connectedHost() == ellaHost })
}

func TestNodeDialsFromTheDiameterInterfaceAddress(t *testing.T) {
	requireSCTP(t)

	configured := netip.MustParseAddr("127.0.0.20")

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: configured.String(), Port: freePort(t)}, zap.NewNop())
	link.SetLocalAddrForTest(func(netip.Addr) (netip.Addr, error) { return loopback, nil })

	runManager(t, link)

	waitFor(t, "link up", func() bool { return smscState(link) == diameter.PeerOpen })

	if got := smsc.connectedAddr(); got != configured {
		t.Fatalf("SMSC saw Ella at %v, want %v", got, configured)
	}
}

func TestDialDeviceFollowsTheDiameterListenerVRF(t *testing.T) {
	errLookup := errors.New("lookup failed")

	tests := []struct {
		name      string
		listen    diameternode.ListenConfig
		device    string
		err       error
		wantCall  []string
		wantDev   string
		wantError bool
	}{
		{name: "no listener", listen: diameternode.ListenConfig{}, device: "vrf-cp"},
		{name: "named interface in a VRF", listen: diameternode.ListenConfig{Name: "eth1", Port: 3868}, device: "vrf-cp", wantCall: []string{"eth1", ""}, wantDev: "vrf-cp"},
		{name: "address in a VRF", listen: diameternode.ListenConfig{Address: "10.0.0.1", Port: 3868}, device: "vrf-cp", wantCall: []string{"", "10.0.0.1"}, wantDev: "vrf-cp"},
		{name: "address outside any VRF", listen: diameternode.ListenConfig{Address: "10.0.0.1", Port: 3868}, wantCall: []string{"", "10.0.0.1"}},
		{name: "unspecified address", listen: diameternode.ListenConfig{Address: "0.0.0.0", Port: 3868}, wantCall: []string{"", "0.0.0.0"}},
		{name: "lookup failure", listen: diameternode.ListenConfig{Name: "eth1", Port: 3868}, err: errLookup, wantCall: []string{"eth1", ""}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called []string

			link := diameternode.New(nil, nil, tt.listen, zap.NewNop())
			link.SetVRFDeviceForTest(func(name, address string) (string, error) {
				called = []string{name, address}
				return tt.device, tt.err
			})

			got, err := link.DialDeviceForTest()
			if (err != nil) != tt.wantError {
				t.Fatalf("dial device error = %v, want error %v", err, tt.wantError)
			}

			if got != tt.wantDev {
				t.Fatalf("dial device = %q, want %q", got, tt.wantDev)
			}

			if !slices.Equal(called, tt.wantCall) {
				t.Fatalf("VRF lookup called with %q, want %q", called, tt.wantCall)
			}
		})
	}
}

func TestNodeDialsThroughTheDiameterVRF(t *testing.T) {
	requireSCTP(t)
	requireBindToDevice(t)

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: loopback.String(), Port: freePort(t)}, zap.NewNop())
	link.SetVRFDeviceForTest(func(string, string) (string, error) { return "lo", nil })

	runManager(t, link)

	waitFor(t, "link up", func() bool { return smscState(link) == diameter.PeerOpen })
}

func TestNodeDoesNotDialAroundTheDiameterVRF(t *testing.T) {
	requireSCTP(t)

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: loopback.String(), Port: freePort(t)}, zap.NewNop())
	link.SetVRFDeviceForTest(func(string, string) (string, error) { return "ella-novrf0", nil })

	runManager(t, link)

	waitFor(t, "the dial to fail", func() bool {
		p, ok := smscPeer(link)
		return ok && p.State != diameter.PeerOpen && p.Error != ""
	})

	if p, _ := smscPeer(link); !strings.Contains(p.Error, syscall.ENODEV.Error()) {
		t.Fatalf("peer error = %q, want %q", p.Error, syscall.ENODEV.Error())
	}
}

func TestNodeDoesNotStartWhenTheDiameterVRFCannotBeResolved(t *testing.T) {
	source := newSettingsSource()
	source.setSMSC(netip.MustParseAddrPort("127.0.0.1:3868"))

	resolved := make(chan struct{}, 1)

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: loopback.String(), Port: freePort(t)}, zap.NewNop())
	link.SetVRFDeviceForTest(func(string, string) (string, error) {
		poke(resolved)
		return "", errors.New("lookup failed")
	})

	runManager(t, link)

	select {
	case <-resolved:
	case <-time.After(waitTimeout):
		t.Fatal("the Diameter VRF was never resolved")
	}

	if link.Node() != nil {
		t.Fatal("a Diameter node started without its VRF")
	}
}

func TestNodeWithAnUnspecifiedDiameterAddressDialsFromTheRouteSource(t *testing.T) {
	requireSCTP(t)

	routed := netip.MustParseAddr("127.0.0.21")

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: "0.0.0.0", Port: freePort(t)}, zap.NewNop())
	link.SetLocalAddrForTest(func(netip.Addr) (netip.Addr, error) { return routed, nil })

	runManager(t, link)

	waitFor(t, "link up", func() bool { return smscState(link) == diameter.PeerOpen })

	if got := smsc.connectedAddr(); got != routed {
		t.Fatalf("SMSC saw Ella at %v, want %v", got, routed)
	}
}

func TestNodeReportsAnSMSCThatAnswersAsAnotherHost(t *testing.T) {
	requireSCTP(t)

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)
	source.smscHost = "other-smsc.example.org"

	link, _ := startManager(t, source)

	want := "peer answered as " + smscHost + ", expected other-smsc.example.org"

	waitFor(t, "the mismatch reported", func() bool {
		p, ok := smscPeer(link)
		return ok && p.State != diameter.PeerOpen && p.Error == want
	})
}

func TestNodeDispatchesRequestsToRegisteredHandlers(t *testing.T) {
	requireSCTP(t)

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link, _ := startManager(t, source)

	waitFor(t, "link up", func() bool { return smscState(link) == diameter.PeerOpen })

	req, err := s6c.NewSendRoutingInfoForSMRequest(tgpp.Envelope{
		SessionID:        smsc.node.NewSessionID(),
		Origin:           smsc.node.Identity(),
		DestinationHost:  ellaHost,
		DestinationRealm: ellaRealm,
	}, s6c.RoutingRequest{MSISDN: "15551230001", ServiceCentreAddress: "15550000000"})
	if err != nil {
		t.Fatalf("build SRR: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ans, err := smsc.node.Send(ctx, req)
	if err != nil {
		t.Fatalf("SRR: %v", err)
	}

	result, err := tgpp.ParseResult(ans)
	if err != nil {
		t.Fatalf("parse SRA: %v", err)
	}

	if result.Code != diameter.ResultUnableToComply {
		t.Fatalf("SRA result = %s, want %d", result, diameter.ResultUnableToComply)
	}

	if _, ok := ans.Find(diameter.AVPAuthSessionState, 0); !ok {
		t.Fatal("SRA has no Auth-Session-State")
	}
}

func TestNodeFollowsSettingsChanges(t *testing.T) {
	requireSCTP(t)

	first := startFakeSMSC(t, 0)
	second := startFakeSMSC(t, 0)

	source := newSettingsSource()
	source.setSMSC(first.addr)

	link, wakeup := startManager(t, source)

	waitFor(t, "link up to the first SMSC", func() bool { return first.connectedHost() == ellaHost })

	node := link.Node()

	source.setSMSC(second.addr)
	poke(wakeup)

	waitFor(t, "link up to the second SMSC", func() bool { return second.connectedHost() == ellaHost })
	waitFor(t, "first SMSC released", func() bool { return first.connectedHost() == "" })

	if link.Node() != node {
		t.Fatal("changing the SMSC rebuilt the Diameter node; it must only replace the peer")
	}

	source.setSMSC(netip.AddrPort{})
	poke(wakeup)

	waitFor(t, "link disabled", func() bool { return len(link.Peers()) == 0 })
	waitFor(t, "second SMSC released", func() bool { return second.connectedHost() == "" })

	if link.Node() != nil {
		t.Fatal("the Diameter node survived disabling SMS")
	}
}

func TestRemovedNodeTellsPeersNotToReconnect(t *testing.T) {
	requireSCTP(t)

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link, wakeup := startManager(t, source)

	waitFor(t, "link up", func() bool { return smsc.connectedHost() == ellaHost })

	source.setNode(diameternode.NodeSettings{}, diameternode.ErrNotClusterMember)
	poke(wakeup)

	waitFor(t, "node stopped", func() bool { return link.Node() == nil })
	waitFor(t, "DO_NOT_WANT_TO_TALK_TO_YOU at the SMSC", func() bool {
		return smsc.peerError() == "the peer disconnected: DO_NOT_WANT_TO_TALK_TO_YOU"
	})
}

func TestNodeReconnectsAfterSMSCRestart(t *testing.T) {
	requireSCTP(t)

	smsc := startFakeSMSC(t, 0)
	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link, _ := startManager(t, source)

	waitFor(t, "link up", func() bool { return smscState(link) == diameter.PeerOpen })

	smsc.stop()

	waitFor(t, "link down", func() bool { return smscState(link) != diameter.PeerOpen })

	restarted := startFakeSMSC(t, int(smsc.addr.Port()))

	waitFor(t, "SMSC to see Ella again", func() bool { return restarted.connectedHost() == ellaHost })

	waitFor(t, "Ella to reopen the link", func() bool {
		state := smscState(link)
		return state == diameter.PeerReopen || state == diameter.PeerOpen
	})
}

func TestNodeReportsTheSMSCDownWhenItCannotStart(t *testing.T) {
	source := newSettingsSource()
	source.setSMSC(netip.MustParseAddrPort("127.0.0.1:3868"))
	source.setNode(diameternode.NodeSettings{MCC: "1", MNC: "01"}, nil)

	link, _ := startManager(t, source)

	waitFor(t, "down state", func() bool { return smscState(link) == diameter.PeerDown })

	if link.Node() != nil {
		t.Fatal("a Diameter node started with an invalid identity")
	}

	if _, err := link.Identity(context.Background()); err == nil {
		t.Fatal("Identity accepted an invalid PLMN")
	}
}

func TestNodeIdentityWithoutSMSC(t *testing.T) {
	source := newSettingsSource()
	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{}, zap.NewNop())

	identity, err := link.Identity(context.Background())
	if err != nil {
		t.Fatalf("Identity: %v", err)
	}

	if identity.Host != ellaHost || identity.Realm != ellaRealm {
		t.Fatalf("identity = %+v", identity)
	}

	source.setNode(diameternode.NodeSettings{}, errors.New("this node has no AMF Pointer yet"))

	if _, err := link.Identity(context.Background()); err == nil {
		t.Fatal("Identity hid a settings error")
	}
}

var (
	smsApplications = []diameter.Application{
		{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
		{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
	}
	loopback2 = netip.MustParseAddr("127.0.0.2")
)

func freePort(t *testing.T) int {
	t.Helper()

	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: loopback.AsSlice()})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	return port
}

func newDialingNode(t *testing.T, host string, ip netip.Addr) *diameter.Node {
	t.Helper()

	node, err := diameter.New(diameter.Config{
		Identity: diameter.Identity{
			OriginHost:      host,
			OriginRealm:     smscRealm,
			HostIPAddresses: []netip.Addr{ip},
			ProductName:     "fake-peer",
		},
		Handler: diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
			return c.Answer(req, diameter.ResultSuccess)
		}),
		HandshakeTimeout:     2 * time.Second,
		ReconnectInterval:    50 * time.Millisecond,
		MaxReconnectInterval: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = node.Shutdown(ctx)
	})

	return node
}

func dialElla(t *testing.T, node *diameter.Node, port int, transport diameter.Transport) {
	t.Helper()

	dialEllaFor(t, node, port, transport, smsApplications)
}

func dialEllaFor(t *testing.T, node *diameter.Node, port int, transport diameter.Transport, apps []diameter.Application) {
	t.Helper()

	err := node.SetPeers([]diameter.Peer{{
		ID:           "ella",
		Addresses:    []netip.Addr{loopback},
		Transports:   []diameter.Transport{transport},
		Dial:         &diameter.Dial{Port: uint16(port)},
		Applications: apps,
	}})
	if err != nil {
		t.Fatalf("set peers: %v", err)
	}
}

func ellaStatus(node *diameter.Node) diameter.PeerStatus {
	for _, p := range node.Peers() {
		if p.ID == "ella" {
			return p
		}
	}

	return diameter.PeerStatus{}
}

func openPeers(node *diameter.Node) int {
	n := 0

	for _, p := range node.Peers() {
		if p.State == diameter.PeerOpen {
			n++
		}
	}

	return n
}

func TestListenerAcceptsAnSMSCThatDialsIn(t *testing.T) {
	requireSCTP(t)

	port := freePort(t)
	smsc := newDialingNode(t, smscHost, loopback)

	source := newSettingsSource()
	source.setSMSC(netip.AddrPortFrom(loopback, uint16(freePort(t))))

	link, _ := startListeningManager(t, source, diameternode.ListenConfig{Address: loopback.String(), Port: port})

	dialElla(t, smsc, port, diameter.TransportSCTP)

	waitFor(t, "SMSC to open towards Ella", func() bool { return ellaStatus(smsc).State == diameter.PeerOpen })
	waitFor(t, "Ella to see the SMSC open", func() bool { return smscState(link) == diameter.PeerOpen })

	if peer, _ := smscPeer(link); peer.Host != smscHost {
		t.Fatalf("SMSC peer host = %q, want %q", peer.Host, smscHost)
	}

	req, err := s6c.NewSendRoutingInfoForSMRequest(tgpp.Envelope{
		SessionID:        smsc.NewSessionID(),
		Origin:           smsc.Identity(),
		DestinationHost:  ellaHost,
		DestinationRealm: ellaRealm,
	}, s6c.RoutingRequest{MSISDN: "15551230001", ServiceCentreAddress: "15550000000"})
	if err != nil {
		t.Fatalf("build SRR: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ans, err := smsc.Send(ctx, req)
	if err != nil {
		t.Fatalf("SRR: %v", err)
	}

	result, err := tgpp.ParseResult(ans)
	if err != nil {
		t.Fatalf("parse SRA: %v", err)
	}

	if result.Code != diameter.ResultUnableToComply {
		t.Fatalf("SRA result = %s, want %d", result, diameter.ResultUnableToComply)
	}
}

func TestListenerAndDialerResolveToOneConnection(t *testing.T) {
	requireSCTP(t)

	port := freePort(t)
	smsc := startFakeSMSC(t, 0)

	source := newSettingsSource()
	source.setSMSC(smsc.addr)

	link, _ := startListeningManager(t, source, diameternode.ListenConfig{Address: loopback.String(), Port: port})

	dialElla(t, smsc.node, port, diameter.TransportSCTP)

	waitFor(t, "link up", func() bool { return smscState(link) == diameter.PeerOpen && smsc.connectedHost() == ellaHost })

	time.Sleep(500 * time.Millisecond)

	if smscState(link) != diameter.PeerOpen || smsc.connectedHost() != ellaHost {
		t.Fatal("connection lost after the election")
	}

	if n := openPeers(smsc.node); n != 1 {
		t.Fatalf("SMSC holds %d open peers, want 1", n)
	}
}

func TestListenerOffersSMSToUnknownPeers(t *testing.T) {
	requireSCTP(t)

	port := freePort(t)

	source := newSettingsSource()
	source.setSMSC(netip.AddrPortFrom(loopback, uint16(freePort(t))))

	startListeningManager(t, source, diameternode.ListenConfig{Address: loopback.String(), Port: port})

	cases := []struct {
		name      string
		ip        netip.Addr
		transport diameter.Transport
	}{
		{"other address over SCTP", loopback2, diameter.TransportSCTP},
		{"SMSC address over TCP", loopback, diameter.TransportTCP},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			other := newDialingNode(t, "other.example.org", tc.ip)
			dialElla(t, other, port, tc.transport)

			waitFor(t, "peer open", func() bool { return ellaStatus(other).State == diameter.PeerOpen })
		})
	}
}

func TestListenerRunsWithoutSMSC(t *testing.T) {
	requireSCTP(t)

	port := freePort(t)

	link, _ := startListeningManager(t, newSettingsSource(), diameternode.ListenConfig{Address: loopback.String(), Port: port})

	waitFor(t, "node started", func() bool { return link.Node() != nil })

	other := newDialingNode(t, "other.example.org", loopback)
	dialElla(t, other, port, diameter.TransportSCTP)

	waitFor(t, "peer open", func() bool { return ellaStatus(other).State == diameter.PeerOpen })
}

func TestSMSCChangesKeepTheIMSPeersConnected(t *testing.T) {
	requireSCTP(t)

	port := freePort(t)
	source := newSettingsSource()
	source.setSMSC(netip.MustParseAddrPort("[2001:db8::1]:3868"))

	link := diameternode.New(source.getNode, source.getPeers, diameternode.ListenConfig{Address: loopback.String(), Port: port}, zap.NewNop())
	link.SetLocalAddrForTest(func(remote netip.Addr) (netip.Addr, error) {
		if remote.Is6() {
			return netip.Addr{}, fmt.Errorf("no route to %s", remote)
		}

		return netip.MustParseAddr("127.0.0.3"), nil
	})

	_, wakeup := runManager(t, link)

	ims := newDialingNode(t, "scscf.ims.mnc001.mcc001.3gppnetwork.org", loopback2)
	dialEllaFor(t, ims, port, diameter.TransportSCTP, imsApplications)

	waitFor(t, "IMS peer open despite the unroutable SMSC", func() bool {
		return ellaStatus(ims).State == diameter.PeerOpen && imsPeer(link).State == diameter.PeerOpen
	})

	if st := smscState(link); st == diameter.PeerOpen {
		t.Fatalf("SMSC state = %s, want it not connected", st)
	}

	node, since := link.Node(), imsPeer(link).Since

	for _, smsc := range []string{"127.0.0.1:3868", "[2001:db8::1]:3868", ""} {
		var addr netip.AddrPort
		if smsc != "" {
			addr = netip.MustParseAddrPort(smsc)
		}

		source.setSMSC(addr)
		poke(wakeup)

		waitFor(t, "the SMSC change applied", func() bool {
			_, ok := smscPeer(link)
			return ok == addr.IsValid()
		})

		if link.Node() != node {
			t.Fatalf("setting the SMSC to %q restarted the Diameter node", smsc)
		}

		if p := imsPeer(link); p.State != diameter.PeerOpen || !p.Since.Equal(since) {
			t.Fatalf("setting the SMSC to %q disturbed the IMS peer: %+v", smsc, p)
		}
	}
}

var imsApplications = []diameter.Application{
	{ID: cx.ApplicationID, VendorID: tgpp.VendorID},
	{ID: rx.ApplicationID, VendorID: tgpp.VendorID},
	{ID: sh.ApplicationID, VendorID: tgpp.VendorID},
	{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
	{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
}

func imsPeer(link *diameternode.Manager) diameternode.PeerStatus {
	for _, p := range link.Peers() {
		if p.Inbound {
			return p
		}
	}

	return diameternode.PeerStatus{}
}

func TestListenerAcceptsUnknownIMSPeersForCxAndRx(t *testing.T) {
	requireSCTP(t)

	const imsHost = "scscf.ims.mnc001.mcc001.3gppnetwork.org"

	for _, transport := range []diameter.Transport{diameter.TransportSCTP, diameter.TransportTCP} {
		t.Run(transport.String(), func(t *testing.T) {
			port := freePort(t)

			link, _ := startListeningManager(t, newSettingsSource(), diameternode.ListenConfig{Address: loopback.String(), Port: port})

			ims := newDialingNode(t, imsHost, loopback2)
			dialEllaFor(t, ims, port, transport, imsApplications)

			waitFor(t, "IMS peer open", func() bool {
				return ellaStatus(ims).State == diameter.PeerOpen && imsPeer(link).State == diameter.PeerOpen
			})

			if apps := ellaStatus(ims).Applications; !slices.Equal(apps, imsApplications) {
				t.Fatalf("negotiated applications = %v, want Cx and Rx", apps)
			}

			if st := imsPeer(link); st.Host != imsHost || st.Address != loopback2.String() {
				t.Fatalf("IMS peer status = %+v", st)
			}

			uar, err := cx.NewUserAuthorizationRequest(tgpp.Envelope{
				SessionID:        ims.NewSessionID(),
				Origin:           ims.Identity(),
				DestinationHost:  ellaHost,
				DestinationRealm: "ims.mnc001.mcc001.3gppnetwork.org",
			}, cx.UserAuthorizationRequest{
				PrivateIdentity: "001010000000001@ims.mnc001.mcc001.3gppnetwork.org",
				PublicIdentity:  "sip:001010000000001@ims.mnc001.mcc001.3gppnetwork.org",
				VisitedNetwork:  "ims.mnc001.mcc001.3gppnetwork.org",
			})
			if err != nil {
				t.Fatalf("build UAR: %v", err)
			}

			str, err := rx.NewSessionTerminationRequest(tgpp.Envelope{
				SessionID:        ims.NewSessionID(),
				Origin:           ims.Identity(),
				DestinationHost:  ellaHost,
				DestinationRealm: ellaRealm,
			}, rx.SessionTerminationRequest{Cause: rx.TerminationLogout})
			if err != nil {
				t.Fatalf("build STR: %v", err)
			}

			for _, req := range []*diameter.Message{uar, str} {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

				ans, err := ims.Send(ctx, req)

				cancel()

				if err != nil {
					t.Fatalf("command %d: %v", req.CommandCode, err)
				}

				result, err := tgpp.ParseResult(ans)
				if err != nil {
					t.Fatalf("parse answer to command %d: %v", req.CommandCode, err)
				}

				if result.Code != diameter.ResultCommandUnsupported {
					t.Fatalf("answer to command %d = %s, want %d", req.CommandCode, result, diameter.ResultCommandUnsupported)
				}
			}
		})
	}
}

func TestServedRealmsFollowTheIMSHomeDomain(t *testing.T) {
	requireSCTP(t)

	const homeDomain = "ims.example.com"

	port := freePort(t)
	source := newSettingsSource()

	link, wakeup := startListeningManager(t, source, diameternode.ListenConfig{Address: loopback.String(), Port: port})

	ims := newDialingNode(t, "scscf.ims.mnc001.mcc001.3gppnetwork.org", loopback2)
	dialEllaFor(t, ims, port, diameter.TransportSCTP, imsApplications)

	waitFor(t, "IMS peer open", func() bool { return ellaStatus(ims).State == diameter.PeerOpen })

	uar := func(realm string) uint32 {
		t.Helper()

		req, err := cx.NewUserAuthorizationRequest(tgpp.Envelope{
			SessionID:        ims.NewSessionID(),
			Origin:           ims.Identity(),
			DestinationRealm: realm,
		}, cx.UserAuthorizationRequest{
			PrivateIdentity: "001010000000001@" + homeDomain,
			PublicIdentity:  "sip:001010000000001@" + homeDomain,
			VisitedNetwork:  homeDomain,
		})
		if err != nil {
			t.Fatalf("build UAR: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ans, err := ims.Do(ctx, "ella", req)
		if err != nil {
			t.Fatalf("UAR to %s: %v", realm, err)
		}

		result, err := tgpp.ParseResult(ans)
		if err != nil {
			t.Fatalf("parse UAA: %v", err)
		}

		return result.Code
	}

	if code := uar(homeDomain); code != diameter.ResultRealmNotServed {
		t.Fatalf("UAR to the home domain before it is configured = %d, want %d", code, diameter.ResultRealmNotServed)
	}

	want := []string{ellaRealm, "ims.mnc001.mcc001.3gppnetwork.org"}
	if got := link.ServedRealms(); !slices.Equal(got, want) {
		t.Fatalf("served realms = %v, want %v", got, want)
	}

	node := link.Node()

	settings := newSettingsSource().node
	settings.HomeDomain = homeDomain
	source.setNode(settings, nil)
	poke(wakeup)

	want = append(want, homeDomain)

	waitFor(t, "the home domain served", func() bool { return slices.Equal(link.ServedRealms(), want) })

	if link.Node() != node {
		t.Fatal("setting the home domain restarted the Diameter node")
	}

	for _, realm := range []string{homeDomain, ellaRealm, "ims.mnc001.mcc001.3gppnetwork.org"} {
		if code := uar(realm); code != diameter.ResultCommandUnsupported {
			t.Fatalf("UAR to %s = %d, want it delivered to the (absent) Cx handler (%d)", realm, code, diameter.ResultCommandUnsupported)
		}
	}

	if ellaStatus(ims).State != diameter.PeerOpen {
		t.Fatal("setting the home domain disturbed the IMS peer")
	}
}

func TestServedRealms(t *testing.T) {
	got, err := diameternode.ServedRealms("001", "01", "IMS.mnc001.mcc001.3gppnetwork.org")
	if err != nil {
		t.Fatalf("ServedRealms: %v", err)
	}

	if want := []string{ellaRealm, "ims.mnc001.mcc001.3gppnetwork.org"}; !slices.Equal(got, want) {
		t.Fatalf("served realms with the derived home domain = %v, want %v", got, want)
	}

	got, _ = diameternode.ServedRealms("001", "01", "example.com")
	if want := []string{ellaRealm, "ims.mnc001.mcc001.3gppnetwork.org", "example.com"}; !slices.Equal(got, want) {
		t.Fatalf("served realms = %v, want %v", got, want)
	}

	if _, err := diameternode.ServedRealms("1", "01", ""); err == nil {
		t.Fatal("an invalid MCC was accepted")
	}
}
