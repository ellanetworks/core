// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/smsf"
)

type stubNode struct {
	node *diameter.Node
}

func (n stubNode) Node() *diameter.Node { return n.node }

func TestTheSMSCIsUnavailableBeforeTheNodeStarts(t *testing.T) {
	c := smsf.PeersOver(stubNode{})

	if _, err := c.Envelope(smsf.SMSCPeerID(smscPeerID)); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("Envelope = %v, want the SMSC unavailable", err)
	}

	if _, err := c.DoAny(t.Context(), []string{smsf.SMSCPeerID(smscPeerID)}, &diameter.Message{}); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("DoAny = %v, want the SMSC unavailable", err)
	}

	if _, ok := c.LocalHost(); ok {
		t.Fatal("a node that has not started reported a local host")
	}
}

func TestTheSMSCEnvelopeTargetsTheNamedPeer(t *testing.T) {
	smsc, err := diameter.New(diameter.Config{
		Identity: diameter.Identity{OriginHost: smscHost, OriginRealm: smscRealm, HostIPAddresses: []netip.Addr{loopback}, ProductName: "fake-smsc"},
		Handler:  diameter.NewMux(),

		AcceptUnknownPeers:      true,
		UnknownPeerApplications: []diameter.Application{{ID: sgd.ApplicationID, VendorID: tgpp.VendorID}},
	})
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: loopback.AsSlice()})
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = smsc.Serve(diameter.NewTCPListener(ln)) }()

	identity := localIdentity
	identity.HostIPAddresses = []netip.Addr{loopback}
	identity.ProductName = "smsf-test"

	node, err := diameter.New(diameter.Config{Identity: identity, Handler: diameter.NewMux()})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = node.Shutdown(context.Background())
		_ = smsc.Shutdown(context.Background())
		_ = ln.Close()
	})

	open, down := smsf.SMSCPeerID("open"), smsf.SMSCPeerID("down")
	apps := []diameter.Application{{ID: sgd.ApplicationID, VendorID: tgpp.VendorID}}

	port, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address %v", ln.Addr())
	}

	if err := node.SetPeers([]diameter.Peer{
		{ID: open, Addresses: []netip.Addr{loopback}, Transports: []diameter.Transport{diameter.TransportTCP}, Dial: &diameter.Dial{Port: uint16(port.Port)}, Applications: apps},
		{ID: down, Addresses: []netip.Addr{netip.MustParseAddr("127.0.0.2")}, Transports: []diameter.Transport{diameter.TransportTCP}, Dial: &diameter.Dial{Port: 1}, Applications: apps},
	}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the SMSC peer opens", func() bool {
		p, ok := node.Peer(open)
		return ok && p.State == diameter.PeerOpen
	})

	c := smsf.PeersOver(stubNode{node: node})

	env, err := c.Envelope(open)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}

	if env.DestinationHost != smscHost || env.DestinationRealm != smscRealm || env.Origin.OriginHost != localIdent.Host || env.SessionID == "" {
		t.Fatalf("envelope = %+v, want it addressed to the open SMSC from %s", env, localIdent.Host)
	}

	if _, err := c.Envelope(down); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("Envelope of a peer that is not connected = %v, want the SMSC unavailable", err)
	}

	if _, err := c.Envelope(smsf.SMSCPeerID("unknown")); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("Envelope of an unknown peer = %v, want the SMSC unavailable", err)
	}

	req := &diameter.Message{Flags: diameter.FlagProxiable, CommandCode: sgd.CommandMOForwardShortMessage, ApplicationID: sgd.ApplicationID, AVPs: env.AVPs()}

	if ans, err := c.DoAny(t.Context(), []string{down, open}, req); err != nil || ans == nil {
		t.Fatalf("DoAny past a peer that is not connected = %v, %v, want an answer from the open SMSC", ans, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	if _, err := c.DoAny(ctx, []string{down}, req); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("DoAny with no open peer = %v, want the SMSC unavailable", err)
	}

	if host, ok := c.LocalHost(); !ok || host != localIdent.Host {
		t.Fatalf("local host = %q %t, want %q", host, ok, localIdent.Host)
	}
}
