// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

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
	c := smsf.SMSCOver(stubNode{})

	if _, err := c.Envelope(smsf.SMSCPeerID(smscPeerID)); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("Envelope = %v, want the SMSC unavailable", err)
	}

	if _, err := c.Do(t.Context(), smsf.SMSCPeerID(smscPeerID), &diameter.Message{}); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("Do = %v, want the SMSC unavailable", err)
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
		{ID: open, Addresses: []netip.Addr{loopback}, Port: uint16(port.Port), Transport: diameter.TransportTCP, Applications: apps},
		{ID: down, Addresses: []netip.Addr{netip.MustParseAddr("127.0.0.2")}, Port: 1, Transport: diameter.TransportTCP, Applications: apps},
	}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the SMSC peer opens", func() bool {
		p, ok := node.Peer(open)
		return ok && p.State == diameter.PeerOpen
	})

	c := smsf.SMSCOver(stubNode{node: node})

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

	if host, ok := c.LocalHost(); !ok || host != localIdent.Host {
		t.Fatalf("local host = %q %t, want %q", host, ok, localIdent.Host)
	}
}
