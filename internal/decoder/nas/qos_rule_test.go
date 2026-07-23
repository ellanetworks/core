// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestUnmarshalQosRules(t *testing.T) {
	qosRulesBytes := fgs.MarshalQoSRules([]fgs.QoSRule{fgs.DefaultQoSRule(1, 1)})

	rules, err := nas.UnmarshalQosRules(qosRulesBytes)
	if err != nil {
		t.Fatal(err)
	}

	if len(rules) != 1 {
		t.Fatalf("Expected 1 QoS Rule, got %d", len(rules))
	}

	if rules[0].Identifier != 1 {
		t.Fatalf("Expected Identifier 1, got %d", rules[0].Identifier)
	}

	if rules[0].OperationCode != 1 {
		t.Fatalf("Expected OperationCode 1, got %d", rules[0].OperationCode)
	}

	if rules[0].DQR.Label != "default" {
		t.Fatalf("Expected DQR 'default', got %s", rules[0].DQR.Label)
	}

	if rules[0].DQR.Value != 1 {
		t.Fatalf("Expected DQR 'default', got %d", rules[0].DQR.Value)
	}

	if rules[0].Precedence != 255 {
		t.Fatalf("Expected Precedence 255, got %d", rules[0].Precedence)
	}

	if rules[0].QFI != 1 {
		t.Fatalf("Expected QFI 1, got %d", rules[0].QFI)
	}

	if len(rules[0].PacketFilterList) != 1 {
		t.Fatalf("Expected 1 Packet Filter, got %d", len(rules[0].PacketFilterList))
	}

	if rules[0].PacketFilterList[0].Identifier != 1 {
		t.Fatalf("Expected Packet Filter Identifier 1, got %d", rules[0].PacketFilterList[0].Identifier)
	}

	if rules[0].PacketFilterList[0].Direction.Label != "bidirectional" {
		t.Fatalf("Expected Packet Filter Direction bidirectional, got %v", rules[0].PacketFilterList[0].Direction.Label)
	}

	if rules[0].PacketFilterList[0].Direction.Value != 0x03 {
		t.Fatalf("Expected Packet Filter Direction bidirectional, got %v", rules[0].PacketFilterList[0].Direction.Value)
	}
}
