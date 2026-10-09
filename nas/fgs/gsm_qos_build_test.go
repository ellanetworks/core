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

func TestPacketFilterComponentsRoundTrip(t *testing.T) {
	rule := QoSRule{
		Identifier:    2,
		OperationCode: QoSRuleOpCreate,
		Parameters:    &QoSRuleParameters{Precedence: 10, QFI: 2},
		Filters: []PacketFilter{{
			Identifier: 1,
			Direction:  PacketFilterBidirectional,
			Components: []PacketFilterComponent{
				RemoteAddressComponent(netip.MustParsePrefix("192.0.2.10/32")),
				ProtocolComponent(17),
				SingleLocalPortComponent(50000),
				SingleRemotePortComponent(49000),
			},
		}, {
			Identifier: 2,
			Direction:  PacketFilterDownlink,
			Components: []PacketFilterComponent{RemoteAddressComponent(netip.MustParsePrefix("2001:db8::10/128"))},
		}},
	}

	b, err := QoSRules{rule}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseQoSRules(b)
	if err != nil || len(got) != 1 || len(got[0].Filters) != 2 || len(got[0].Filters[0].Components) != 4 {
		t.Fatalf("parsed %+v (%v), want the rule back with its components", got, err)
	}
}
