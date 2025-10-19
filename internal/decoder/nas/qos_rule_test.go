package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/smf/qos"
)

func TestUnmarshalQosRules(t *testing.T) {
	qosRule := &qos.QosRule{
		Identifier:    1,
		DQR:           0x01,
		OperationCode: qos.OperationCodeCreateNewQoSRule,
		Precedence:    255,
		QFI:           1,
		PacketFilterList: []qos.PacketFilter{
			{
				Identifier: 1,
				Direction:  qos.PacketFilterDirectionBidirectional,
				Content: []qos.PacketFilterComponent{{
					ComponentType: qos.PFComponentTypeMatchAll,
				}},
				ContentLength: 0x01,
			},
		},
	}

	qosRulesBytes, err := qosRule.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

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

	if rules[0].DQR != "default" {
		t.Fatalf("Expected DQR 'default', got %s", rules[0].DQR)
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

	if rules[0].PacketFilterList[0].Direction != "bidirectional" {
		t.Fatalf("Expected Packet Filter Direction bidirectional, got %v", rules[0].PacketFilterList[0].Direction)
	}
}
