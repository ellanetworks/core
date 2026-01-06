// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/smf/nas"
)

func TestBuildDefaultQosRule_Marshal(t *testing.T) {
	qosRules := nas.BuildDefaultQosRule(1, 1)

	bytes, err := qosRules.MarshalBinary()
	if err != nil {
		t.Errorf("Error marshaling QoS Rules: %v", err)
	}

	expectedBytes := []byte{0x01, 0x00, 0x06, 0x31, 0x31, 0x01, 0x01, 0xff, 0x01}
	if string(expectedBytes) != string(bytes) {
		t.Errorf("Expected: %v, Got: %v", expectedBytes, bytes)
	}
}

func TestBuildDefaultQosRule_Content(t *testing.T) {
	qosFlow := nas.BuildDefaultQosRule(1, 1)

	if qosFlow.Identifier != 1 {
		t.Errorf("Expected Identifier 1, got %d", qosFlow.Identifier)
	}

	if qosFlow.PacketFilterList[0].Identifier != 1 {
		t.Errorf("Expected PacketFilterList Identifier 1, got %d", qosFlow.PacketFilterList[0].Identifier)
	}
}
