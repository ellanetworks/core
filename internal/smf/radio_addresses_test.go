// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/radioctx"
)

func TestRadioAddressesRecordTheRadioThatBoundTheSession(t *testing.T) {
	source := AnchorBinding{TEID: 1, IPv4: net.ParseIP("10.3.0.11").To4()}
	target := AnchorBinding{TEID: 2, IPv4: net.ParseIP("10.3.0.12").To4()}
	enb := AnchorBinding{TEID: 3, IPv4: net.ParseIP("192.168.40.10").To4()}

	fromSource := radioctx.With(context.Background(), "gnb:001-01:000001@22")
	fromTarget := radioctx.With(context.Background(), "gnb:001-01:000002@22")

	s := &SMF{}

	s.radios.recordSwitched(fromSource, dataPlane{}, dataPlane{AN: source})
	s.radios.recordSwitched(fromTarget, dataPlane{AN: source}, dataPlane{AN: source})
	s.radios.recordSwitched(context.Background(), dataPlane{AN: source}, dataPlane{AN: target})
	s.radios.recordSwitched(radioctx.With(context.Background(), "enb:001-01:000003"),
		dataPlane{AN: enb}, dataPlane{AN: enb, Bearers: []bearerLeg{{Slot: 1, AN: enb}}})

	got := s.RadioIDsByAddress()

	if id := got[netip.MustParseAddr("10.3.0.11")]; id != "gnb:001-01:000001@22" {
		t.Errorf("source gNB address = %q, want the radio that bound it, not the handover target that sent a later message", id)
	}

	if _, ok := got[netip.MustParseAddr("10.3.0.12")]; ok {
		t.Error("an address was recorded without a radio on the request")
	}

	if id := got[netip.MustParseAddr("192.168.40.10")]; id != "enb:001-01:000003" {
		t.Errorf("eNB address = %q, want the dedicated bearer's radio", id)
	}
}
