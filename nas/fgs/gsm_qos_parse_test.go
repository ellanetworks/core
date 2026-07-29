// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"reflect"
	"testing"
)

func TestQoSRulesRoundTrip(t *testing.T) {
	rules := []QoSRule{
		DefaultQoSRule(1, 1),
		{
			Identifier: 2, OperationCode: 1, DQR: 0, Precedence: 100, QFI: 5, Segregation: 1,
			Filters: []PacketFilter{
				{Identifier: 3, Direction: 2, Components: []PacketFilterComponent{
					{Type: 0x10, Value: []byte{8, 8, 8, 8, 255, 255, 255, 255}}, // IPv4 addr+mask
					{Type: 0x30, Value: []byte{0x06}},                           // protocol id
				}},
			},
		},
	}

	got, err := ParseQoSRules(mustBytes(QoSRules(rules).MarshalBinary()))
	if err != nil {
		t.Fatalf("ParseQoSRules: %v", err)
	}

	if !reflect.DeepEqual(got, QoSRules(rules)) {
		t.Fatalf("round-trip:\n got %+v\nwant %+v", got, rules)
	}
}

func TestQoSFlowDescriptionsRoundTrip(t *testing.T) {
	got, err := ParseQoSFlowDescriptions(mustBytes(QoSFlowDescriptions{FiveQIQoSFlow(3, 9, QoSFlowOpCreate)}.MarshalBinary()))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(got) != 1 || got[0].QFI != 3 || got[0].OperationCode != QoSFlowOpCreate || !got[0].EBit {
		t.Fatalf("framing: %+v", got)
	}

	if len(got[0].Parameters) != 1 || got[0].Parameters[0].ID != QoSFlowParam5QI ||
		len(got[0].Parameters[0].Value) != 1 || got[0].Parameters[0].Value[0] != 9 {
		t.Fatalf("params: %+v", got[0].Parameters)
	}
}

func TestQoSFlowParameterKbps(t *testing.T) {
	if v, ok := (QoSFlowParameter{ID: QoSFlowParamMFBRDownlink, Value: []byte{qosRateUnit1Mbps, 0x02, 0x00}}).Kbps(); !ok || v != 512000 {
		t.Fatalf("Mbps decode = %d,%v", v, ok)
	}
}
