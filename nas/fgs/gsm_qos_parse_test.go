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
			Identifier: 2, OperationCode: 1, DQR: 0, Parameters: &QoSRuleParameters{Precedence: 100, QFI: 5, Segregation: 1},
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

// TestQoSRuleParametersAreConditional checks the rule precedence and
// segregation/QFI octets against TS 24.501 §9.11.4.13: "create new QoS rule"
// must carry them, "delete existing QoS rule" must not, and the four modify
// operations round-trip either way.
func TestQoSRuleParametersAreConditional(t *testing.T) {
	modifyOps := []QoSRuleOperation{
		QoSRuleOpModifyAddFilters,
		QoSRuleOpModifyReplaceFilters,
		QoSRuleOpModifyDeleteFilters,
		QoSRuleOpModifyWithoutFilters,
	}

	for _, op := range modifyOps {
		for _, params := range []*QoSRuleParameters{nil, {Precedence: 7, QFI: 5}} {
			rules := QoSRules{{Identifier: 2, OperationCode: op, Parameters: params}}

			b, err := rules.MarshalBinary()
			if err != nil {
				t.Fatalf("%s with parameters %v: MarshalBinary: %v", op, params, err)
			}

			got, err := ParseQoSRules(b)
			if err != nil {
				t.Fatalf("%s with parameters %v: ParseQoSRules(% x): %v", op, params, b, err)
			}

			if !reflect.DeepEqual(got, rules) {
				t.Errorf("%s: round trip:\n got %+v\nwant %+v", op, got, rules)
			}
		}
	}

	if _, err := (QoSRules{{Identifier: 2, OperationCode: QoSRuleOpCreate}}).MarshalBinary(); err == nil {
		t.Error("encoded a create operation with no precedence and no QFI")
	}

	if _, err := (QoSRules{{Identifier: 2, OperationCode: QoSRuleOpDelete, Parameters: &QoSRuleParameters{}}}).MarshalBinary(); err == nil {
		t.Error("encoded a delete operation carrying a precedence and a QFI")
	}

	// A create operation whose content stops after the packet filter list.
	if _, err := ParseQoSRules([]byte{0x02, 0x00, 0x01, 0x20}); err == nil {
		t.Error("parsed a create operation with no precedence and no QFI")
	}

	// A delete operation carrying the two octets it must omit.
	if _, err := ParseQoSRules([]byte{0x02, 0x00, 0x03, 0x40, 0x07, 0x05}); err == nil {
		t.Error("parsed a delete operation carrying a precedence and a QFI")
	}
}
