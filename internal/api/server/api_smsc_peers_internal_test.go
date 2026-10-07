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

func TestSMSCPeerResponseCarriesItsLinkStatus(t *testing.T) {
	since := time.Date(2026, 9, 29, 12, 0, 0, 0, time.FixedZone("EDT", -4*3600))

	statuses := smscPeerStatuses(fakeDiameterNode{peers: []diameternode.PeerStatus{{
		ID:      "smsc-a",
		Role:    "smsc",
		Host:    "smsc.example.org",
		Realm:   "example.org",
		Address: netip.MustParseAddrPort("192.0.2.10:3868"),
		State:   diameter.PeerOpen,
		Since:   since,
	}}})

	got := smscPeerResponse(db.SMSCPeer{ID: "a", DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", Port: 3868, ServiceCentres: []string{"15550000000"}}, statuses)
	want := SMSCPeer{
		ID: "a", DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", Port: 3868, ServiceCentres: []string{"+15550000000"},
		Status: &SMSCPeerStatus{State: "open", Host: "smsc.example.org", Realm: "example.org", Since: "2026-09-29T16:00:00Z"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("peer = %+v, want %+v", got, want)
	}

	if other := smscPeerResponse(db.SMSCPeer{ID: "b", DiameterIdentity: "smsc-192-0-2-11.example.org", Address: "192.0.2.11", Port: 3868}, statuses); other.Status != nil {
		t.Fatalf("a peer this node does not track has status %+v", other.Status)
	}

	if none := smscPeerStatuses(nil); len(none) != 0 {
		t.Fatalf("statuses without a node = %+v", none)
	}
}
