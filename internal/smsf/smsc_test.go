// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/smsf"
)

type stubNode struct {
	node  *diameter.Node
	peers []diameternode.PeerStatus
}

func (n stubNode) Node() *diameter.Node             { return n.node }
func (n stubNode) Peers() []diameternode.PeerStatus { return n.peers }

func TestTheSMSCIsUnavailableBeforeTheNodeStarts(t *testing.T) {
	c := smsf.SMSCOver(stubNode{})

	if _, err := c.Envelope(); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("Envelope = %v, want the SMSC unavailable", err)
	}

	if _, err := c.Do(t.Context(), &diameter.Message{}); !errors.Is(err, smsf.ErrSMSCUnavailable) {
		t.Fatalf("Do = %v, want the SMSC unavailable", err)
	}

	if _, ok := c.LocalHost(); ok {
		t.Fatal("a node that has not started reported a local host")
	}
}

func TestTheSMSCEnvelopeTargetsTheOpenSMSCPeer(t *testing.T) {
	identity := localIdentity
	identity.HostIPAddresses = []netip.Addr{loopback}
	identity.ProductName = "smsf-test"

	node, err := diameter.New(diameter.Config{Identity: identity, Handler: diameter.NewMux()})
	if err != nil {
		t.Fatal(err)
	}

	c := smsf.SMSCOver(stubNode{node: node, peers: []diameternode.PeerStatus{
		{Role: "hss", Host: "hss.example.org", Realm: smscRealm, State: diameter.PeerOpen},
		{Role: smsf.PeerRoleSMSC, Host: "down.example.org", Realm: smscRealm},
		{Role: smsf.PeerRoleSMSC, Host: smscHost, Realm: smscRealm, State: diameter.PeerOpen},
	}})

	env, err := c.Envelope()
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}

	if env.DestinationHost != smscHost || env.DestinationRealm != smscRealm || env.Origin.OriginHost != localIdent.Host || env.SessionID == "" {
		t.Fatalf("envelope = %+v, want it addressed to the open SMSC from %s", env, localIdent.Host)
	}

	if host, ok := c.LocalHost(); !ok || host != localIdent.Host {
		t.Fatalf("local host = %q %t, want %q", host, ok, localIdent.Host)
	}
}
