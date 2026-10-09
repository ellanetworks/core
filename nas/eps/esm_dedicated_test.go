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

func remoteAddress(t *testing.T, prefix string) eps.TFTComponent {
	t.Helper()

	c, err := eps.RemoteAddress(netip.MustParsePrefix(prefix))
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func voiceTFT(t *testing.T) eps.TrafficFlowTemplate {
	t.Helper()

	remote := remoteAddress(t, "10.60.0.9/32")

	return eps.TrafficFlowTemplate{
		Operation: eps.TFTCreate,
		Filters: []eps.TFTPacketFilter{
			{Identifier: 1, Direction: eps.TFTBidirectional, Precedence: 10, Components: []eps.TFTComponent{
				remote, eps.ProtocolIdentifier(17), eps.SingleLocalPort(31000), eps.SingleRemotePort(32000),
			}},
			{Identifier: 2, Direction: eps.TFTBidirectional, Precedence: 11, Components: []eps.TFTComponent{
				remote, eps.ProtocolIdentifier(17), eps.SingleLocalPort(31001), eps.SingleRemotePort(32001),
			}},
		},
	}
}

func TestTFTEncoding(t *testing.T) {
	tft := eps.TrafficFlowTemplate{
		Operation: eps.TFTCreate,
		Filters: []eps.TFTPacketFilter{{Identifier: 1, Direction: eps.TFTUplink, Precedence: 0x0A, Components: []eps.TFTComponent{
			remoteAddress(t, "192.0.2.0/24"), eps.ProtocolIdentifier(17), eps.RemotePortRange(1000, 2000),
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
	c := remoteAddress(t, "2001:db8::1/64")

	want := append(netip.MustParseAddr("2001:db8::").AsSlice(), netip.MustParseAddr("ffff:ffff:ffff:ffff::").AsSlice()...)
	if c.Type != eps.TFTIPv6RemoteAddress || !bytes.Equal(c.Value, want) {
		t.Fatalf("component = %#x % x, want IPv6 remote address % x", uint8(c.Type), c.Value, want)
	}
}

func TestTFTRemoteAddressUnmapsIPv4(t *testing.T) {
	c := remoteAddress(t, "::ffff:192.0.2.10/128")

	want := []byte{192, 0, 2, 10, 255, 255, 255, 255}
	if c.Type != eps.TFTIPv4RemoteAddress || !bytes.Equal(c.Value, want) {
		t.Fatalf("component = %#x % x, want IPv4 remote address % x", uint8(c.Type), c.Value, want)
	}
}

func TestTFTRemoteAddressRejectsInvalidPrefix(t *testing.T) {
	if _, err := eps.RemoteAddress(netip.Prefix{}); err == nil {
		t.Fatal("invalid prefix encoded, want an error")
	}
}

func TestTFTRejectsMalformedTemplates(t *testing.T) {
	for name, tft := range map[string]eps.TrafficFlowTemplate{
		"create without filters":   {Operation: eps.TFTCreate},
		"delete existing with one": {Operation: eps.TFTDeleteExisting, Filters: voiceTFT(t).Filters[:1]},
		"repeated component": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{Identifier: 1, Components: []eps.TFTComponent{
			eps.ProtocolIdentifier(17), eps.ProtocolIdentifier(6),
		}}}},
		"short component": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{Identifier: 1, Components: []eps.TFTComponent{
			{Type: eps.TFTSingleLocalPort, Value: []byte{1}},
		}}}},
		"repeated identifier": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{
			{Identifier: 1, Components: []eps.TFTComponent{eps.ProtocolIdentifier(17)}},
			{Identifier: 1, Components: []eps.TFTComponent{eps.ProtocolIdentifier(6)}},
		}},
		"single and range local port": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{Identifier: 1, Components: []eps.TFTComponent{
			eps.SingleLocalPort(5060), eps.LocalPortRange(5060, 5070),
		}}}},
		"single and range remote port": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{Identifier: 1, Components: []eps.TFTComponent{
			eps.SingleRemotePort(5060), eps.RemotePortRange(5060, 5070),
		}}}},
		"IPv4 and IPv6 remote address": {Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{{Identifier: 1, Components: []eps.TFTComponent{
			remoteAddress(t, "192.0.2.1/32"), remoteAddress(t, "2001:db8::1/128"),
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
		{
			rates: eps.EPSQoSBitRates{MaxUplinkKbps: 300_000, MaxDownlinkKbps: 1_000_000, GuaranteedUplinkKbps: 2_000_000, GuaranteedDownlinkKbps: 1000},
			want:  []byte{1, 0xFE, 0xFE, 0xFE, 0x86, 0xFA, 0xFA, 0xFA, 0, 0x0B, 0x6F, 0xA6, 0},
			read:  eps.EPSQoSBitRates{MaxUplinkKbps: 300_000, MaxDownlinkKbps: 1_000_000, GuaranteedUplinkKbps: 2_000_000, GuaranteedDownlinkKbps: 960},
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

	if _, err := eps.GBREPSQoS(1, eps.EPSQoSBitRates{MaxDownlinkKbps: 10_000_001}); err == nil {
		t.Fatal("a rate above 10 Gbit/s encoded, want an error")
	}

	if _, err := eps.GBREPSQoS(1, eps.EPSQoSBitRates{GuaranteedUplinkKbps: 64}); err == nil {
		t.Fatal("a 0 kbit/s maximum bit rate in both directions encoded, want an error")
	}
}

func TestActivateDedicatedEPSBearerContextRoundTrip(t *testing.T) {
	qos, err := eps.GBREPSQoS(1, eps.EPSQoSBitRates{MaxUplinkKbps: 41, MaxDownlinkKbps: 41, GuaranteedUplinkKbps: 41, GuaranteedDownlinkKbps: 41})
	if err != nil {
		t.Fatal(err)
	}

	cause := eps.ESMCause(44)

	for _, msg := range []eps.Message{
		&eps.ActivateDedicatedEPSBearerContextRequest{EPSBearerIdentity: 6, PTI: 0, LinkedEPSBearerIdentity: 5, EPSQoS: qos, TFT: voiceTFT(t)},
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

	wire, err := (&eps.ActivateDedicatedEPSBearerContextRequest{EPSBearerIdentity: 6, PTI: nas.ProcedureTransactionIdentity(0), LinkedEPSBearerIdentity: 5, EPSQoS: qos, TFT: voiceTFT(t)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if wire[0] != 0x62 || wire[2] != 0xC5 || wire[3] != 0x05 || wire[4] != 5 || wire[5] != 1 {
		t.Fatalf("header = % x, want EBI 6, ESM, type C5, linked EBI 5, EPS QoS length 5 QCI 1", wire[:6])
	}
}

func TestParseTFTRejectsMalformedWire(t *testing.T) {
	for name, wire := range map[string][]byte{
		"delete existing with a filter count": {0x43},
		"no operation with a filter count":    {0xC1},
		"E bit without parameters":            {0x50},
		"repeated identifier": {
			0x22,
			0x31, 0x00, 0x02, 0x30, 17,
			0x31, 0x01, 0x02, 0x30, 6,
		},
		"single and range local port": {
			0x21,
			0x31, 0x00, 0x08, 0x40, 0x13, 0xC4, 0x41, 0x13, 0xC4, 0x13, 0xCE,
		},
	} {
		if _, err := eps.ParseTrafficFlowTemplate(wire); err == nil {
			t.Errorf("%s: parsed % x, want an error", name, wire)
		}
	}
}

func TestParseTFTIgnoreOperation(t *testing.T) {
	tft, err := eps.ParseTrafficFlowTemplate([]byte{0x1F, 0xAA, 0xBB})
	if err != nil || tft.Operation != eps.TFTIgnore || tft.Filters != nil || tft.Parameters != nil {
		t.Fatalf("parsed %+v, %v; want an empty ignore TFT", tft, err)
	}
}

func TestParseTFTEthernetComponents(t *testing.T) {
	wire := []byte{
		0x21,
		0x31, 0x00, 0x16,
		0x81, 0x02, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x82, 0x02, 0x00, 0x00, 0x00, 0x00, 0x02,
		0x83, 0x00, 0x64,
		0x85, 0x0A,
		0x87, 0x88, 0xF7,
	}

	tft, err := eps.ParseTrafficFlowTemplate(wire)
	if err != nil {
		t.Fatal(err)
	}

	want := []eps.TFTComponentType{eps.TFTDestinationMACAddress, eps.TFTSourceMACAddress, eps.TFTCTagVID, eps.TFTCTagPCPDEI, eps.TFTEthertype}
	if len(tft.Filters) != 1 || len(tft.Filters[0].Components) != len(want) {
		t.Fatalf("parsed %+v, want one filter with %d components", tft, len(want))
	}

	for i, c := range tft.Filters[0].Components {
		if c.Type != want[i] {
			t.Fatalf("component %d = %#x, want %#x", i, uint8(c.Type), uint8(want[i]))
		}
	}

	back, err := tft.MarshalBinary()
	if err != nil || !bytes.Equal(back, wire) {
		t.Fatalf("re-encoded % x, %v; want % x", back, err, wire)
	}
}
