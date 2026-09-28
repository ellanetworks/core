// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/sctp"
)

const (
	testAppID    uint32 = 16777313
	otherAppID   uint32 = 16777217
	testVendorID uint32 = 10415
	testTimeout         = 5 * time.Second
)

var (
	loopback1 = netip.MustParseAddr("127.0.0.1")
	loopback2 = netip.MustParseAddr("127.0.0.2")
	sgdApp    = Application{ID: testAppID, VendorID: testVendorID}
	otherApp  = Application{ID: otherAppID, VendorID: testVendorID}
)

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

func testConfig(host string) Config {
	return Config{
		Identity: Identity{
			OriginHost:      host,
			OriginRealm:     "example.org",
			HostIPAddresses: []netip.Addr{loopback1},
			VendorID:        testVendorID,
			ProductName:     "test",
		},
		Handler: HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
			return c.Answer(req, ResultSuccess)
		}),
		HandshakeTimeout:     2 * time.Second,
		ReconnectInterval:    50 * time.Millisecond,
		MaxReconnectInterval: 200 * time.Millisecond,
		noWatchdogJitter:     true,
		Logger:               testLogger(),
	}
}

func newTestNode(t *testing.T, cfg Config) *Node {
	t.Helper()

	n, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		_ = n.Shutdown(ctx)
	})

	return n
}

func serveOn(t *testing.T, n *Node, kind Transport, ip netip.Addr) netip.AddrPort {
	t.Helper()

	var ln Listener

	switch kind {
	case TransportSCTP:
		requireSCTP(t)

		var lc sctp.ListenConfig

		sl, err := lc.Listen(context.Background(), &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: ip.AsSlice()}}})
		if err != nil {
			t.Fatalf("Listen: %v", err)
		}

		ln = NewSCTPListener(sl, nil)
	default:
		t.Fatalf("unsupported transport %s", kind)
	}

	served := make(chan error, 1)

	go func() { served <- n.Serve(ln) }()

	t.Cleanup(func() {
		_ = ln.Close()

		<-served
	})

	return listenerAddr(ln)
}

func listenerAddr(ln Listener) netip.AddrPort {
	switch a := ln.Addr().(type) {
	case *sctp.SCTPAddr:
		ip, _ := netip.AddrFromSlice(a.IPAddrs[0].IP)
		return netip.AddrPortFrom(ip.Unmap(), uint16(a.Port))
	default:
		panic("unknown listener address")
	}
}

type peerEvent struct {
	m   *Message
	err error
}

type rawPeer struct {
	t      *testing.T
	tr     transport
	events chan peerEvent
}

func dialRaw(t *testing.T, kind Transport, local netip.Addr, to netip.AddrPort) *rawPeer {
	t.Helper()

	p, err := tryDialRaw(t, kind, local, to)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	return p
}

func tryDialRaw(t *testing.T, kind Transport, local netip.Addr, to netip.AddrPort) (*rawPeer, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var (
		tr  transport
		err error
	)

	switch kind {
	case TransportSCTP:
		requireSCTP(t)

		tr, err = dialSCTP(ctx, []netip.Addr{local}, []netip.Addr{to.Addr()}, to.Port(), nil)
	default:
		t.Fatalf("unsupported transport %s", kind)
	}

	if err != nil {
		return nil, err
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

			if err != nil {
				return
			}
		}
	}()

	t.Cleanup(func() { _ = tr.abort() })

	return p, nil
}

func (p *rawPeer) send(m *Message) {
	p.t.Helper()

	b, err := m.Marshal()
	if err != nil {
		p.t.Fatalf("Marshal: %v", err)
	}

	if err := p.tr.writeMessage(b); err != nil {
		p.t.Fatalf("write: %v", err)
	}
}

func (p *rawPeer) recv() *Message {
	p.t.Helper()

	select {
	case e := <-p.events:
		if e.err != nil {
			p.t.Fatalf("read: %v", e.err)
		}

		return e.m
	case <-time.After(testTimeout):
		p.t.Fatal("timed out waiting for a message")
	}

	return nil
}

func (p *rawPeer) expectClosed() {
	p.t.Helper()

	deadline := time.After(testTimeout)

	for {
		select {
		case e := <-p.events:
			if e.err != nil {
				return
			}
		case <-deadline:
			p.t.Fatal("timed out waiting for the connection to close")
		}
	}
}

func cer(host string, apps ...AVP) *Message {
	return &Message{
		Flags:       FlagRequest,
		CommandCode: CommandCapabilitiesExchange,
		HopByHopID:  1,
		EndToEndID:  1,
		AVPs: append([]AVP{
			UTF8String(AVPOriginHost, AVPFlagMandatory, 0, host),
			UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "example.org"),
			Address(AVPHostIPAddress, AVPFlagMandatory, 0, loopback1),
			Unsigned32(AVPVendorID, AVPFlagMandatory, 0, 0),
			UTF8String(AVPProductName, 0, 0, "test"),
		}, apps...),
	}
}

func appAVP(app Application) AVP {
	return Grouped(AVPVendorSpecificApplicationID, AVPFlagMandatory, 0,
		Unsigned32(AVPVendorID, AVPFlagMandatory, 0, app.VendorID),
		Unsigned32(AVPAuthApplicationID, AVPFlagMandatory, 0, app.ID),
	)
}

func appRequest(hopByHop, endToEnd uint32, origin string, extra ...AVP) *Message {
	return &Message{
		Flags:         FlagRequest,
		CommandCode:   8388645,
		ApplicationID: testAppID,
		HopByHopID:    hopByHop,
		EndToEndID:    endToEnd,
		AVPs: append([]AVP{
			UTF8String(AVPSessionID, AVPFlagMandatory, 0, origin+";1"),
			UTF8String(AVPOriginHost, AVPFlagMandatory, 0, origin),
			UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "example.org"),
		}, extra...),
	}
}

func resultCode(t *testing.T, m *Message) uint32 {
	t.Helper()

	a, ok := m.Find(AVPResultCode, 0)
	if !ok {
		t.Fatalf("command %d has no Result-Code", m.CommandCode)
	}

	v, err := a.Unsigned32()
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func advertisedApplications(t *testing.T, m *Message) map[uint32]bool {
	t.Helper()

	return commonApplications(m.AVPs, []Application{sgdApp, otherApp, {ID: RelayApplicationID}})
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}

func request() *Message {
	return &Message{
		CommandCode:   8388647,
		ApplicationID: testAppID,
		AVPs: []AVP{
			UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "x"),
			UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "example.org"),
		},
	}
}

func doOK(n *Node, peerID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ans, err := n.Do(ctx, peerID, request())
	if err != nil {
		return false
	}

	rc, ok := ans.Find(AVPResultCode, 0)
	v, _ := rc.Unsigned32()

	return ok && v == ResultSuccess
}

func isTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

var transports = []Transport{TransportSCTP}

func testLogger() *slog.Logger {
	if os.Getenv("DIAMETER_TEST_LOG") != "" {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	return slog.New(slog.DiscardHandler)
}
