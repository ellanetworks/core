// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"bytes"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func voiceTFT() eps.TrafficFlowTemplate {
	remote := netip.MustParsePrefix("10.60.0.9/32")

	return eps.TrafficFlowTemplate{
		Operation: eps.TFTCreate,
		Filters: []eps.TFTPacketFilter{
			{Identifier: 1, Direction: eps.TFTBidirectional, Precedence: 10, Components: []eps.TFTComponent{
				eps.RemoteAddress(remote), eps.ProtocolIdentifier(17), eps.SingleLocalPort(31000), eps.SingleRemotePort(32000),
			}},
			{Identifier: 2, Direction: eps.TFTBidirectional, Precedence: 11, Components: []eps.TFTComponent{
				eps.RemoteAddress(remote), eps.ProtocolIdentifier(17), eps.SingleLocalPort(31001), eps.SingleRemotePort(32001),
			}},
		},
	}
}

func TestTFTEncoding(t *testing.T) {
	tft := eps.TrafficFlowTemplate{
		Operation: eps.TFTCreate,
		Filters: []eps.TFTPacketFilter{{Identifier: 1, Direction: eps.TFTUplink, Precedence: 0x0A, Components: []eps.TFTComponent{
			eps.RemoteAddress(netip.MustParsePrefix("192.0.2.0/24")), eps.ProtocolIdentifier(17), eps.RemotePortRange(1000, 2000),
		}}},
	}

	got, err := tft.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x21,
		0x21, 0x0A, 0x10,
		0x10, 192, 0, 2, 0, 255, 255, 255, 0,
		0x30, 17,
		0x51, 0x03, 0xE8, 0x07, 0xD0,
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("TFT = % x, want % x", got, want)
	}

	back, err := eps.ParseTrafficFlowTemplate(got)
	if err != nil || !reflect.DeepEqual(back, tft) {
		t.Fatalf("parsed %+v, %v; want %+v", back, err, tft)
	}
}

func TestTFTIPv6RemoteAddress(t *testing.T) {
	c := eps.RemoteAddress(netip.MustParsePrefix("2001:db8::/64"))

	want := append(netip.MustParseAddr("2001:db8::").AsSlice(), 64)
	if c.Type != eps.TFTIPv6RemoteAddressPrefix || !bytes.Equal(c.Value, want) {
		t.Fatalf("component = %#x % x, want IPv6 remote address/prefix length % x", uint8(c.Type), c.Value, want)
	}
}

func TestTFTRejectsMalformedTemplates(t *testing.T) {
	for name, tft := range map[string]eps.TrafficFlowTemplate{
		"create without filters":   {Operation: eps.TFTCreate},
		"delete existing with one": {Operation: eps.TFTDeleteExisting, Filters: voiceTFT().Filters[:1]},
		"repeated component": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{Identifier: 1, Components: []eps.TFTComponent{
			eps.ProtocolIdentifier(17), eps.ProtocolIdentifier(6),
		}}}},
		"short component": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{Identifier: 1, Components: []eps.TFTComponent{
			{Type: eps.TFTSingleLocalPort, Value: []byte{1}},
		}}}},
	} {
		if _, err := tft.MarshalBinary(); err == nil {
			t.Errorf("%s: encoded, want an error", name)
		}
	}
}

func TestGBREPSQoS(t *testing.T) {
	for _, tc := range []struct {
		rates eps.EPSQoSBitRates
		want  []byte
		read  eps.EPSQoSBitRates
	}{
		{
			rates: eps.EPSQoSBitRates{MaxUplinkKbps: 41, MaxDownlinkKbps: 41, GuaranteedUplinkKbps: 41, GuaranteedDownlinkKbps: 41},
			want:  []byte{1, 41, 41, 41, 41},
			read:  eps.EPSQoSBitRates{MaxUplinkKbps: 41, MaxDownlinkKbps: 41, GuaranteedUplinkKbps: 41, GuaranteedDownlinkKbps: 41},
		},
		{
			rates: eps.EPSQoSBitRates{MaxUplinkKbps: 100, MaxDownlinkKbps: 0, GuaranteedUplinkKbps: 70, GuaranteedDownlinkKbps: 0},
			want:  []byte{1, 0x44, 0xFF, 0x40, 0xFF},
			read:  eps.EPSQoSBitRates{MaxUplinkKbps: 96, GuaranteedUplinkKbps: 64},
		},
		{
			rates: eps.EPSQoSBitRates{MaxUplinkKbps: 20_000, MaxDownlinkKbps: 20_000, GuaranteedUplinkKbps: 1000, GuaranteedDownlinkKbps: 1000},
			want:  []byte{1, 0xFE, 0xFE, 0x86, 0x86, 0x4E, 0x4E, 0, 0},
			read:  eps.EPSQoSBitRates{MaxUplinkKbps: 20_000, MaxDownlinkKbps: 20_000, GuaranteedUplinkKbps: 960, GuaranteedDownlinkKbps: 960},
		},
	} {
		q, err := eps.GBREPSQoS(1, tc.rates)
		if err != nil {
			t.Fatalf("GBREPSQoS(%+v): %v", tc.rates, err)
		}

		got, err := q.MarshalBinary()
		if err != nil || !bytes.Equal(got, tc.want) {
			t.Fatalf("GBREPSQoS(%+v) = % x, %v; want % x", tc.rates, got, err, tc.want)
		}

		if read, ok := q.GBRBitRates(); !ok || read != tc.read {
			t.Fatalf("GBRBitRates() = %+v, %t; want %+v", read, ok, tc.read)
		}
	}

	if _, err := eps.GBREPSQoS(1, eps.EPSQoSBitRates{MaxDownlinkKbps: 300_000}); err == nil {
		t.Fatal("a 300 Mbit/s rate encoded, want an error")
	}
}

func TestActivateDedicatedEPSBearerContextRoundTrip(t *testing.T) {
	qos, err := eps.GBREPSQoS(1, eps.EPSQoSBitRates{MaxUplinkKbps: 41, MaxDownlinkKbps: 41, GuaranteedUplinkKbps: 41, GuaranteedDownlinkKbps: 41})
	if err != nil {
		t.Fatal(err)
	}

	cause := eps.ESMCause(44)

	for _, msg := range []eps.Message{
		&eps.ActivateDedicatedEPSBearerContextRequest{EPSBearerIdentity: 6, PTI: 0, LinkedEPSBearerIdentity: 5, EPSQoS: qos, TFT: voiceTFT()},
		&eps.ActivateDedicatedEPSBearerContextAccept{EPSBearerIdentity: 6, PTI: 0},
		&eps.ActivateDedicatedEPSBearerContextReject{EPSBearerIdentity: 6, PTI: 0, Cause: cause},
	} {
		wire, err := msg.MarshalBinary()
		if err != nil {
			t.Fatalf("%T: %v", msg, err)
		}

		parsed, err := eps.ParseMessage(wire, nas.DirectionDownlink)
		if err != nil {
			t.Fatalf("%T: parse: %v", msg, err)
		}

		if !reflect.DeepEqual(parsed, msg) {
			t.Fatalf("%T round trip = %+v, want %+v", msg, parsed, msg)
		}
	}
}

func TestActivateDedicatedEPSBearerContextRequestLayout(t *testing.T) {
	qos, err := eps.GBREPSQoS(1, eps.EPSQoSBitRates{MaxUplinkKbps: 41, MaxDownlinkKbps: 41, GuaranteedUplinkKbps: 41, GuaranteedDownlinkKbps: 41})
	if err != nil {
		t.Fatal(err)
	}

	wire, err := (&eps.ActivateDedicatedEPSBearerContextRequest{EPSBearerIdentity: 6, PTI: nas.ProcedureTransactionIdentity(0), LinkedEPSBearerIdentity: 5, EPSQoS: qos, TFT: voiceTFT()}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if wire[0] != 0x62 || wire[2] != 0xC5 || wire[3] != 0x05 || wire[4] != 5 || wire[5] != 1 {
		t.Fatalf("header = % x, want EBI 6, ESM, type C5, linked EBI 5, EPS QoS length 5 QCI 1", wire[:6])
	}
}
