// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/roaming"
)

func TestRoamerViewMergesTheLiveContextOfEachNode(t *testing.T) {
	partnerID := "p1"
	imsi := "208930000000001"
	since := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	view := &roamerView{
		partners: map[string]db.RoamingPartner{partnerID: {ID: partnerID, Name: "partner", PLMNs: []db.RoamingPLMN{{Mcc: "208", Mnc: "93"}}}},
		regs: map[string][]db.UERegistration{imsi: {
			{Imsi: imsi, Type: db.UERegistrationTypeAMF3GPPAccess, NodeID: "n1"},
			{Imsi: imsi, Type: db.UERegistrationTypeMME, NodeID: "n2"},
		}},
		nodes: map[string]nodeRoamers{
			"n1": {nodeID: "n1", roamers: []roaming.LiveRoamer{{
				IMSI: imsi, PartnerID: partnerID, RAT: roaming.RAT5G, Registered: true, Connected: true, Since: since, LastSeenAt: since, Radio: "gnb-1",
				Sessions: []roaming.LiveRoamerSession{{ID: 1, DataNetwork: "internet", IPv4Address: "10.45.0.2"}},
			}}},
			"n2": {nodeID: "n2", err: errors.New("unreachable")},
		},
		usage:   map[string]RoamerUsage{imsi: {UplinkBytes: 1, DownlinkBytes: 2, TotalBytes: 3}},
		withUse: true,
	}

	got := view.roamer(db.Subscriber{Imsi: imsi, RoamingPartnerID: &partnerID})

	if got.PartnerName != "partner" || got.HomePLMN != (RoamingPLMN{Mcc: "208", Mnc: "93"}) || got.UsageToday == nil || got.UsageToday.TotalBytes != 3 {
		t.Fatalf("roamer = %+v", got)
	}

	want5G := RoamerRegistration{RAT: "5G", NodeID: "n1", Reachable: true, Registered: true, ConnectionState: "connected", Since: "2026-11-03T10:00:00Z", LastSeenAt: "2026-11-03T10:00:00Z", Radio: "gnb-1"}
	if len(got.Registrations) != 2 || got.Registrations[0] != want5G || got.Registrations[1] != (RoamerRegistration{RAT: "4G", NodeID: "n2"}) {
		t.Fatalf("registrations = %+v", got.Registrations)
	}

	if len(got.Sessions) != 1 || got.Sessions[0] != (RoamerSession{RAT: "5G", NodeID: "n1", ID: 1, DataNetwork: "internet", IPv4Address: "10.45.0.2"}) {
		t.Fatalf("sessions = %+v", got.Sessions)
	}

	if unreachable := view.unreachable(); len(unreachable) != 1 || unreachable[0].NodeID != "n2" {
		t.Fatalf("unreachable = %+v", unreachable)
	}

	view.withUse = false

	if got := view.roamer(db.Subscriber{Imsi: imsi, RoamingPartnerID: &partnerID}); got.UsageToday != nil {
		t.Fatalf("usage shown without permission: %+v", got.UsageToday)
	}
}
