// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

// TS 23.501 §5.8.2.7
func TestQoSFlowFARCarriesItsOwnMarking(t *testing.T) {
	_, conn := newBearerTestEngine(t, 8)

	ctx := context.Background()

	const seid = uint64(61)

	ohc := &models.OuterHeaderCreation{
		Description: models.OuterHeaderCreationGtpUUdpIpv4,
		TEID:        0x99,
		IPv4Address: net.ParseIP("10.3.0.9").To4(),
	}

	session := &models.FAR{
		FARID:       2,
		ApplyAction: models.ApplyAction{Forw: true},
		ForwardingParameters: &models.ForwardingParameters{
			DestinationInterface:  models.InterfaceAccess,
			TransportLevelMarking: 0xA0FC,
			OuterHeaderCreation:   ohc,
		},
	}

	voice := &models.FAR{
		FARID:       48,
		ApplyAction: models.ApplyAction{Forw: true},
		ForwardingParameters: &models.ForwardingParameters{
			DestinationInterface:  models.InterfaceAccess,
			TransportLevelMarking: 0xB8FC,
			OuterHeaderCreation:   ohc,
		},
	}

	if _, err := conn.EstablishSession(ctx, &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000001",
		FARs: []models.FAR{*session, *voice},
		QERs: []models.QER{{QERID: 1, QFI: 2}},
		URRs: []models.URR{{URRID: 1}},
		PDRs: []models.PDR{
			{PDRID: 2, Precedence: 256, FARID: 2, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.61")}},
			{PDRID: 256, Precedence: 32, FARID: 48, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.61"), SDFFilters: []models.SDFFilter{{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50000}}}},
		},
	}); err != nil {
		t.Fatalf("establish: %v", err)
	}

	if got := conn.GetSession(seid).GetFar(2).TransportLevelMarking; got != 0xA0FC {
		t.Errorf("session FAR marking = %#04x, want 0xA0FC", got)
	}

	if got := conn.GetSession(seid).GetFar(48).TransportLevelMarking; got != 0xB8FC {
		t.Errorf("flow FAR marking = %#04x, want 0xB8FC", got)
	}

	if got := conn.GetSession(seid).GetPDR(256).PdrInfo.Far.TransportLevelMarking; got != 0xB8FC {
		t.Errorf("the flow's downlink PDR carries FAR marking %#04x, want 0xB8FC", got)
	}

	relabelled := *voice
	relabelled.ForwardingParameters.TransportLevelMarking = 0xA0FC

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		UpdateFARs: []models.FAR{relabelled},
	}); err != nil {
		t.Fatalf("relabel the flow FAR: %v", err)
	}

	if got := conn.GetSession(seid).GetPDR(256).PdrInfo.Far.TransportLevelMarking; got != 0xA0FC {
		t.Errorf("the flow's downlink PDR carries FAR marking %#04x after the relabel, want 0xA0FC", got)
	}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		RemovePDRs: []uint16{256},
		RemoveFARs: []uint32{48},
	}); err != nil {
		t.Fatalf("remove the flow FAR: %v", err)
	}

	if got := conn.GetSession(seid).GetFar(48); got.TransportLevelMarking != 0 || got.TeID != 0 {
		t.Errorf("the removed flow FAR still holds %+v", got)
	}
}
