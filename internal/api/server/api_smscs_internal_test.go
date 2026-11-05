// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
)

func TestSMSCResponseCarriesTheLinkStatusOfEachNode(t *testing.T) {
	since := time.Date(2026, 9, 29, 12, 0, 0, 0, time.FixedZone("EDT", -4*3600))

	statuses := smscPeerStatuses(fakeDiameterNode{peers: []diameternode.PeerStatus{
		{
			ID:      "smsc-a",
			Role:    "smsc",
			Host:    "smsc.example.org",
			Realm:   "example.org",
			Address: netip.MustParseAddrPort("192.0.2.10:3868"),
			State:   diameter.PeerOpen,
			Since:   since,
		},
		{
			ID: "smsc-c", State: diameter.PeerDown, Since: since, Error: "peer answered as smsc.example.org, expected smsc-c.example.org",
		},
	}})

	got := smscResponse(db.SMSC{
		ID:             "s",
		Name:           "main",
		ServiceCentres: []string{"15550000000"},
		Nodes: []db.SMSCNode{
			{ID: "a", DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", Port: 3868},
			{ID: "b", DiameterIdentity: "smsc-192-0-2-11.example.org", Address: "192.0.2.11", Port: 3868},
			{ID: "c", Address: "192.0.2.12", Port: 3868},
		},
	}, statuses)

	want := SMSC{
		Name:           "main",
		ServiceCentres: []string{"+15550000000"},
		Nodes: []SMSCNode{
			{
				DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", Port: 3868,
				Status: &SMSCNodeStatus{State: "open", Host: "smsc.example.org", Realm: "example.org", Since: "2026-09-29T16:00:00Z"},
			},
			{DiameterIdentity: "smsc-192-0-2-11.example.org", Address: "192.0.2.11", Port: 3868},
			{
				Address: "192.0.2.12", Port: 3868,
				Status: &SMSCNodeStatus{State: "down", Since: "2026-09-29T16:00:00Z", Error: "peer answered as smsc.example.org, expected smsc-c.example.org"},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SMSC = %+v, want %+v", got, want)
	}

	if none := smscPeerStatuses(nil); len(none) != 0 {
		t.Fatalf("statuses without a node = %+v", none)
	}
}

func TestSMSCUpdateKeepsTheIDOfAnUnchangedNode(t *testing.T) {
	existing := []db.SMSCNode{{ID: "kept", Address: "192.0.2.10", Port: 3868}}

	smsc := db.SMSC{ID: "s", Name: "main"}

	if message := smscFromParams(&smsc, []string{"+15550000000"}, []SMSCNodeParams{
		{DiameterIdentity: "smsc1.example.org", Address: "192.0.2.10"},
		{Address: "::ffff:192.0.2.11", Port: 3869},
	}, existing); message != "" {
		t.Fatalf("smscFromParams: %s", message)
	}

	if smsc.Nodes[0].ID != "kept" || smsc.Nodes[0].Port != 3868 || smsc.Nodes[0].DiameterIdentity != "smsc1.example.org" {
		t.Fatalf("first node = %+v, want the existing ID and the default port", smsc.Nodes[0])
	}

	if smsc.Nodes[1].ID == "" || smsc.Nodes[1].ID == "kept" || smsc.Nodes[1].Address != "192.0.2.11" {
		t.Fatalf("second node = %+v, want a new ID and an unmapped address", smsc.Nodes[1])
	}

	if err := smsc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
