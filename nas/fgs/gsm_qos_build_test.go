// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"net/netip"
	"testing"
)

func TestBitRateQoSFlowParameterRoundsUpIntoTheSmallestUnit(t *testing.T) {
	tests := []struct {
		bps  uint64
		want []byte
	}{
		{41000, []byte{qosRateUnit1Kbps, 0, 41}},
		{2050, []byte{qosRateUnit1Kbps, 0, 3}},
		{70_000_000, []byte{0x02, 0x44, 0x5C}},
	}

	for _, tc := range tests {
		p, err := BitRateQoSFlowParameter(QoSFlowParamGFBRUplink, tc.bps)
		if err != nil || !bytes.Equal(p.Value, tc.want) {
			t.Errorf("%d bps encodes as %x (%v), want %x", tc.bps, p.Value, err, tc.want)
		}

		if kbps, ok := p.Kbps(); !ok || kbps*1000 < tc.bps {
			t.Errorf("%d bps decodes back to %d kbps, want at least the requested rate", tc.bps, kbps)
		}
	}
}

func mustRemoteAddressComponent(t *testing.T, prefix string) PacketFilterComponent {
	t.Helper()

	c, err := RemoteAddressComponent(netip.MustParsePrefix(prefix))
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func TestPacketFilterComponentsEncoding(t *testing.T) {
	rule := QoSRule{
		Identifier:    2,
		OperationCode: QoSRuleOpCreate,
		Parameters:    &QoSRuleParameters{Precedence: 10, QFI: 2},
		Filters: []PacketFilter{{
			Identifier: 1,
			Direction:  PacketFilterBidirectional,
			Components: []PacketFilterComponent{
				mustRemoteAddressComponent(t, "192.0.2.10/32"),
				ProtocolComponent(17),
				SingleLocalPortComponent(50000),
				SingleRemotePortComponent(49000),
			},
		}, {
			Identifier: 2,
			Direction:  PacketFilterDownlink,
			Components: []PacketFilterComponent{mustRemoteAddressComponent(t, "2001:db8::10/128")},
		}},
	}

	b, err := QoSRules{rule}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x02, 0x00, 0x2A,
		0x22,
		0x31, 0x11,
		0x10, 192, 0, 2, 10, 0xFF, 0xFF, 0xFF, 0xFF,
		0x30, 17,
		0x40, 0xC3, 0x50,
		0x50, 0xBF, 0x68,
		0x12, 0x12,
		0x21, 0x20, 0x01, 0x0D, 0xB8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 128,
		0x0A,
		0x02,
	}
	if !bytes.Equal(b, want) {
		t.Fatalf("QoS rules = % x, want % x", b, want)
	}

	got, err := ParseQoSRules(b)
	if err != nil || len(got) != 1 || len(got[0].Filters) != 2 || len(got[0].Filters[0].Components) != 4 {
		t.Fatalf("parsed %+v (%v), want the rule back with its components", got, err)
	}
}

func TestRemoteAddressComponentUnmapsIPv4(t *testing.T) {
	c := mustRemoteAddressComponent(t, "::ffff:192.0.2.10/128")

	want := []byte{192, 0, 2, 10, 0xFF, 0xFF, 0xFF, 0xFF}
	if c.Type != pfComponentTypeIPv4RemoteAddress || !bytes.Equal(c.Value, want) {
		t.Fatalf("component = %#x % x, want IPv4 remote address % x", uint8(c.Type), c.Value, want)
	}
}

func TestRemoteAddressComponentRejectsInvalidPrefix(t *testing.T) {
	if _, err := RemoteAddressComponent(netip.Prefix{}); err == nil {
		t.Fatal("invalid prefix encoded, want an error")
	}
}

func TestQoSRuleFilterCountPerOperation(t *testing.T) {
	filter := PacketFilter{Identifier: 1, Direction: PacketFilterBidirectional, Components: []PacketFilterComponent{ProtocolComponent(17)}}
	params := &QoSRuleParameters{Precedence: 10, QFI: 2}

	for _, tc := range []struct {
		op      QoSRuleOperation
		filters []PacketFilter
		ok      bool
	}{
		{QoSRuleOpCreate, nil, true},
		{QoSRuleOpModifyReplaceFilters, nil, true},
		{QoSRuleOpModifyWithoutFilters, nil, true},
		{QoSRuleOpModifyWithoutFilters, []PacketFilter{filter}, false},
		{QoSRuleOpModifyAddFilters, nil, false},
		{QoSRuleOpModifyAddFilters, []PacketFilter{filter}, true},
		{QoSRuleOpModifyDeleteFilters, nil, false},
		{QoSRuleOpModifyDeleteFilters, []PacketFilter{{Identifier: 1}}, true},
	} {
		_, err := QoSRules{{Identifier: 1, OperationCode: tc.op, Parameters: params, Filters: tc.filters}}.MarshalBinary()
		if (err == nil) != tc.ok {
			t.Errorf("%s with %d filters: err = %v, want ok %t", tc.op, len(tc.filters), err, tc.ok)
		}
	}

	for name, wire := range map[string][]byte{
		"modify without filters, one filter": {0x01, 0x00, 0x06, 0xC1, 0x31, 0x02, 0x30, 17, 0x0A},
		"add filters, none":                  {0x01, 0x00, 0x03, 0x60, 0x0A, 0x02},
		"delete filters, none":               {0x01, 0x00, 0x03, 0xA0, 0x0A, 0x02},
	} {
		if _, err := ParseQoSRules(wire); err == nil {
			t.Errorf("%s: parsed % x, want an error", name, wire)
		}
	}
}
