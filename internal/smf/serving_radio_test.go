// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/radioctx"
)

func TestBindingsCarryTheRadioThatServesThem(t *testing.T) {
	const (
		sourceRadio = "gnb:001-01:000001@22"
		targetRadio = "gnb:001-01:000002@22"
		enbRadio    = "enb:001-01:000003"
	)

	source := AnchorBinding{TEID: 1, IPv4: net.ParseIP("10.3.0.11").To4()}
	target := AnchorBinding{TEID: 2, IPv4: net.ParseIP("10.3.0.12").To4()}
	enb := AnchorBinding{TEID: 3, IPv4: net.ParseIP("192.168.40.10").To4()}

	fromSource := radioctx.With(context.Background(), sourceRadio)
	fromTarget := radioctx.With(context.Background(), targetRadio)

	bound := dataPlane{AN: source}.withServingRadios(fromSource, dataPlane{})
	if bound.AN.RadioID != sourceRadio {
		t.Fatalf("new binding radio = %q, want the radio whose message bound it, %q", bound.AN.RadioID, sourceRadio)
	}

	prepared := dataPlane{AN: source}.withServingRadios(fromTarget, bound)
	if prepared.AN.RadioID != sourceRadio {
		t.Errorf("unchanged binding radio = %q after a message from the handover target, want %q", prepared.AN.RadioID, sourceRadio)
	}

	switched := dataPlane{AN: target}.withServingRadios(fromTarget, prepared)
	if switched.AN.RadioID != targetRadio {
		t.Errorf("switched binding radio = %q, want the target %q", switched.AN.RadioID, targetRadio)
	}

	if anonymous := (dataPlane{AN: target}).withServingRadios(context.Background(), bound); anonymous.AN.RadioID != "" {
		t.Errorf("binding switched without a radio on the request = %q, want none", anonymous.AN.RadioID)
	}

	if idle := (dataPlane{}).withServingRadios(fromSource, bound); idle.AN.RadioID != "" {
		t.Errorf("released binding radio = %q, want none", idle.AN.RadioID)
	}

	fourG := dataPlane{AN: enb, Access: Access4G, Downlink: DownlinkForwarding, Bearers: []bearerLeg{{Slot: 1, AN: enb, Rules: []ruleLeg{{Index: 0, Filters: []models.SDFFilter{{Direction: models.FilterDownlink}}}}}}}
	fourG.UEIPv4 = netip.MustParseAddr("10.0.0.1")
	fourG = fourG.withServingRadios(radioctx.With(context.Background(), enbRadio), dataPlane{})

	if fourG.Bearers[0].AN.RadioID != enbRadio {
		t.Errorf("dedicated bearer radio = %q, want %q", fourG.Bearers[0].AN.RadioID, enbRadio)
	}

	_, fars, _, _ := fourG.rules() //nolint:dogsled // only the FARs are under test

	tunnels := 0

	for _, far := range fars {
		fp := far.ForwardingParameters
		if fp == nil || fp.OuterHeaderCreation == nil {
			continue
		}

		tunnels++

		if fp.OuterHeaderCreation.RadioID != enbRadio {
			t.Errorf("FAR %d tunnel radio = %q, want %q", far.FARID, fp.OuterHeaderCreation.RadioID, enbRadio)
		}
	}

	if tunnels != 2 {
		t.Errorf("%d tunnel FARs, want the default bearer's and the dedicated bearer's", tunnels)
	}
}
