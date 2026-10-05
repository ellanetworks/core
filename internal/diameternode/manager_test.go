// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
)

const (
	smscHost    = "smsc.example.org"
	smscRealm   = "example.org"
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
}

func startFakeSMSC(t *testing.T, port int) *fakeSMSC {
	t.Helper()

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

	s := &fakeSMSC{node: node, ln: ln, addr: netip.AddrPortFrom(loopback, uint16(a.Port))}

	t.Cleanup(s.stop)

	return s
}

func (s *fakeSMSC) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.node.Shutdown(ctx)
	_ = s.ln.Close()
}

func (s *fakeSMSC) connectedHost() string {
	for _, p := range s.node.Peers() {
		if p.State == diameter.PeerOpen {
			return p.Host
		}
	}

	return ""
}

type settingsSource struct {
	mu      sync.Mutex
	smsc    netip.AddrPort
	node    diameternode.NodeSettings
	nodeErr error
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
		Role:    "smsc",
		Address: s.smsc,
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

	link := diameternode.New(source.getNode, source.getPeers, listen, zap.NewNop())
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
	if peer.Host != smscHost || peer.Realm != smscRealm || peer.Address != smsc.addr || peer.Since.IsZero() {
		t.Fatalf("peer = %+v, want %s / %s at %s", peer, smscHost, smscRealm, smsc.addr)
	}

	waitFor(t, "SMSC to see Ella", func() bool { return smsc.connectedHost() == ellaHost })
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
		DestinationRealm: ellaRealm,
	}, s6c.RoutingRequest{MSISDN: "15551230001", ServiceCentreAddress: "15550000000"})
	if err != nil {
		t.Fatalf("build SRR: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ans, err := smsc.node.DoHost(ctx, ellaHost, req)
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

	if state := smscState(link); state != diameter.PeerReopen && state != diameter.PeerOpen {
		t.Fatalf("state after reconnecting = %s, want reopen until the RFC 3539 watchdogs complete", state)
	}
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

	err := node.SetPeers([]diameter.Peer{{
		ID:           "ella",
		Addresses:    []netip.Addr{loopback},
		Port:         uint16(port),
		Transport:    transport,
		Applications: smsApplications,
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
		DestinationRealm: ellaRealm,
	}, s6c.RoutingRequest{MSISDN: "15551230001", ServiceCentreAddress: "15550000000"})
	if err != nil {
		t.Fatalf("build SRR: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ans, err := smsc.DoHost(ctx, ellaHost, req)
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

func TestListenerRefusesPeersOtherThanTheSMSC(t *testing.T) {
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

			waitFor(t, "unknown peer refusal", func() bool {
				return strings.Contains(ellaStatus(other).LastError, strconv.Itoa(int(diameter.ResultUnknownPeer)))
			})
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

	waitFor(t, "unknown peer refusal", func() bool {
		return strings.Contains(ellaStatus(other).LastError, strconv.Itoa(int(diameter.ResultUnknownPeer)))
	})
}
