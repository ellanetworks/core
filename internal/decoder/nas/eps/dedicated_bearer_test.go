// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func TestDecodeActivateDedicatedBearer(t *testing.T) {
	qos, err := eps.GBREPSQoS(1, eps.EPSQoSBitRates{MaxUplinkKbps: 41, MaxDownlinkKbps: 41, GuaranteedUplinkKbps: 41, GuaranteedDownlinkKbps: 41})
	if err != nil {
		t.Fatal(err)
	}

	v4, err := eps.RemoteAddress(netip.MustParsePrefix("10.60.0.9/32"))
	if err != nil {
		t.Fatal(err)
	}

	v6, err := eps.RemoteAddress(netip.MustParsePrefix("2001:db8::/64"))
	if err != nil {
		t.Fatal(err)
	}

	b, err := (&eps.ActivateDedicatedEPSBearerContextRequest{
		EPSBearerIdentity:       6,
		LinkedEPSBearerIdentity: 5,
		EPSQoS:                  qos,
		TFT: eps.TrafficFlowTemplate{Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{
			Identifier: 1, Direction: eps.TFTBidirectional, Precedence: 10,
			Components: []eps.TFTComponent{v4, eps.ProtocolIdentifier(17), eps.RemotePortRange(32000, 32001)},
		}, {
			Identifier: 2, Direction: eps.TFTDownlink, Precedence: 11,
			Components: []eps.TFTComponent{v6},
		}}},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeEPSNASMessage(b)
	if msg.ESMMessage == nil || msg.ESMMessage.ActivateDedicatedBearer == nil {
		t.Fatalf("decoded %+v, want an activate dedicated bearer", msg.ESMMessage)
	}

	d := msg.ESMMessage.ActivateDedicatedBearer
	if d.LinkedEPSBearerIdentity != 5 || d.EPSQoS.QCI != 1 || d.EPSQoS.GuaranteedDownlinkKbps == nil || *d.EPSQoS.GuaranteedDownlinkKbps != 41 {
		t.Fatalf("activate dedicated bearer = %+v, QoS %+v", d, d.EPSQoS)
	}

	if len(d.TFT.PacketFilters) != 2 || d.TFT.Operation.Label != "Create new TFT" {
		t.Fatalf("TFT = %+v", d.TFT)
	}

	comps := d.TFT.PacketFilters[0].Components
	if len(comps) != 3 || comps[0].Value != "10.60.0.9/32" || comps[1].Value != "17" || comps[2].Value != "32000-32001" {
		t.Fatalf("components = %+v", comps)
	}

	if v6comps := d.TFT.PacketFilters[1].Components; len(v6comps) != 1 || v6comps[0].Value != "2001:db8::/64" {
		t.Fatalf("IPv6 components = %+v", v6comps)
	}
}
