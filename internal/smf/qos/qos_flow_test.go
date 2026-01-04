// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/qos"
)

func TestBuildAuthorizedQosFlowDescriptions(t *testing.T) {
	smPolicyDecision := &models.SmPolicyDecision{
		QosDecs: &models.QosData{
			QFI:           1,
			Var5qi:        5,
			MaxbrUl:       "101 Mbps",
			MaxbrDl:       "201 Mbps",
			PriorityLevel: 5,
		},
	}

	smPolicyUpdates := qos.BuildSmPolicyUpdate(smPolicyDecision)

	authorizedQosFlow, err := qos.BuildAuthorizedQosFlowDescription(smPolicyUpdates.QosFlowUpdate)
	if err != nil {
		t.Fatalf("Error building Authorized QoS Flow Descriptions: %v", err)
	}

	expectedBytes := []byte{
		0x1, 0x20, 0x43, 0x1, 0x1, 0x5, 0x4, 0x3, 0x6, 0x0, 0x65, 0x5, 0x3, 0x6, 0x0, 0xc9,
	}
	if string(expectedBytes) != string(authorizedQosFlow.Content) {
		t.Errorf("Expected: %v, got: %v", expectedBytes, authorizedQosFlow.Content)
	}
}
