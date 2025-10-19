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
	// make SM Policy Decision
	smPolicyDecision := &models.SmPolicyDecision{}

	// make Sm ctxt Policy Data
	smCtxtPolData := &qos.SmCtxtPolicyData{}

	smPolicyDecision.QosDecs = &models.QosData{
		QFI:                  1,
		Var5qi:               5,
		MaxbrUl:              "101 Mbps",
		MaxbrDl:              "201 Mbps",
		GbrUl:                "11 Mbps",
		GbrDl:                "21 Mbps",
		PriorityLevel:        5,
		DefQosFlowIndication: true,
	}

	smPolicyUpdates := qos.BuildSmPolicyUpdate(smCtxtPolData, smPolicyDecision)

	authorizedQosFlow, err := qos.BuildAuthorizedQosFlowDescription(smPolicyUpdates.QosFlowUpdate.Add)
	if err != nil {
		t.Errorf("Error building Authorized QoS Flow Descriptions: %v", err)
		return
	}

	expectedBytes := []byte{
		0x1, 0x20, 0x45, 0x1, 0x1, 0x5, 0x4, 0x3, 0x6, 0x0,
		0x65, 0x5, 0x3, 0x6, 0x0, 0xc9, 0x2, 0x3, 0x6, 0x0, 0xb, 0x3, 0x3, 0x6,
		0x0, 0x15,
	}
	if string(expectedBytes) != string(authorizedQosFlow.Content) {
		t.Errorf("Expected: %v, got: %v", expectedBytes, authorizedQosFlow.Content)
	}
}
