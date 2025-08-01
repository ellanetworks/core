// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/qos"
)

func TestBuildQosRules(t *testing.T) {
	// make SM Policy Decision
	smPolicyDecision := &models.SmPolicyDecision{}

	// make Sm ctxt Policy Data
	smCtxtPolData := &qos.SmCtxtPolicyData{}

	smPolicyDecision.PccRules = makeSamplePccRules()
	smPolicyDecision.QosDecs = makeSampleQosData()
	smPolicyDecision.SessRules = makeSampleSessionRule()

	smPolicyUpdates := qos.BuildSmPolicyUpdate(smCtxtPolData, smPolicyDecision)

	qosRules, err := qos.BuildQosRules(smPolicyUpdates)
	if err != nil {
		t.Errorf("Error building QoS Rules: %v", err)
		return
	}

	if bytes, err := qosRules.MarshalBinary(); err != nil {
		t.Errorf("Error marshaling QoS Rules: %v", err)
	} else {
		// The order of the bytes may vary because we have 2 rules and maps are used to store them.
		// So we check if the bytes match either of the expected byte arrays.
		expectedBytes1 := []byte{0x01, 0x00, 0x37, 0x32, 0x31, 0x18, 0x10, 0x01, 0x01, 0x01, 0x01, 0xff, 0xff, 0xff, 0xff, 0x50, 0x03, 0xe8, 0x11, 0x02, 0x02, 0x02, 0x02, 0xff, 0xff, 0xff, 0xff, 0x40, 0x07, 0xd0, 0x32, 0x18, 0x10, 0x03, 0x03, 0x03, 0x03, 0xff, 0xff, 0xff, 0xff, 0x50, 0x0b, 0xb8, 0x11, 0x04, 0x04, 0x04, 0x04, 0xff, 0xff, 0xff, 0xff, 0x40, 0x0f, 0xa0, 0xc8, 0x05, 0xff, 0x00, 0x06, 0x31, 0xff, 0x01, 0x01, 0xff, 0x08}
		expectedBytes2 := []byte{0x01, 0x00, 0x37, 0x32, 0x31, 0x18, 0x10, 0x01, 0x01, 0x01, 0x01, 0xff, 0xff, 0xff, 0xff, 0x50, 0x03, 0xe8, 0x11, 0x02, 0x02, 0x02, 0x02, 0xff, 0xff, 0xff, 0xff, 0x40, 0x07, 0xd0, 0x32, 0x18, 0x10, 0x03, 0x03, 0x03, 0x03, 0xff, 0xff, 0xff, 0xff, 0x50, 0x0b, 0xb8, 0x11, 0x04, 0x04, 0x04, 0x04, 0xff, 0xff, 0xff, 0xff, 0x40, 0x0f, 0xa0, 0xc8, 0x05, 0xff, 0x00, 0x06, 0x31, 0xff, 0x01, 0x01, 0xff, 0x09}
		if string(expectedBytes1) != string(bytes) && string(expectedBytes2) != string(bytes) {
			t.Errorf("Expected: %v, Got: %v", expectedBytes1, bytes)
		}
	}
}

func makeSamplePccRules() map[string]*models.PccRule {
	pccRule1 := models.PccRule{
		PccRuleID:  "1",
		Precedence: 200,
		RefQosData: []string{"QosData1"},
		FlowInfos:  make([]models.FlowInformation, 0),
	}

	flowInfos := []models.FlowInformation{
		{
			FlowDescription:   "permit out ip from 1.1.1.1 1000 to 2.2.2.2 2000",
			PackFiltID:        "1",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRmBidirectional,
		},
		{
			FlowDescription:   "permit out ip from 3.3.3.3 3000 to 4.4.4.4 4000",
			PackFiltID:        "2",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRmBidirectional,
		},
	}

	pccRule1.FlowInfos = append(pccRule1.FlowInfos, flowInfos...)

	return map[string]*models.PccRule{"PccRule1": &pccRule1}
}

func makeSampleQosData() map[string]*models.QosData {
	qosData1 := models.QosData{
		QosID:                "5",
		Var5qi:               5,
		MaxbrUl:              "101 Mbps",
		MaxbrDl:              "201 Mbps",
		GbrUl:                "11 Mbps",
		GbrDl:                "21 Mbps",
		PriorityLevel:        5,
		DefQosFlowIndication: true,
	}

	/*
		qosData2 := models.QosData{
			QosID:                "QosData2",
			Var5qi:               3,
			MaxbrUl:              "301 Mbps",
			MaxbrDl:              "401 Mbps",
			GbrUl:                "31 Mbps",
			GbrDl:                "41 Mbps",
			PriorityLevel:        3,
			DefQosFlowIndication: false,
		}
	*/

	return map[string]*models.QosData{
		"QosData1": &qosData1,
		//		"QosData2": &qosData2,
	}
}

func makeSampleSessionRule() map[string]*models.SessionRule {
	sessRule1 := models.SessionRule{
		AuthSessAmbr: &models.Ambr{
			Uplink:   "77 Mbps",
			Downlink: "99 Mbps",
		},
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: 9,
			Arp: &models.Arp{
				PriorityLevel: 8,
				PreemptCap:    models.PreemptionCapabilityMayPreempt,
				PreemptVuln:   models.PreemptionVulnerabilityNotPreemptable,
			},
			PriorityLevel: 8,
		},
	}
	sessRule2 := models.SessionRule{
		AuthSessAmbr: &models.Ambr{
			Uplink:   "55 Mbps",
			Downlink: "33 Mbps",
		},
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: 8,
			Arp: &models.Arp{
				PriorityLevel: 7,
				PreemptCap:    models.PreemptionCapabilityMayPreempt,
				PreemptVuln:   models.PreemptionVulnerabilityNotPreemptable,
			},
			PriorityLevel: 7,
		},
	}

	return map[string]*models.SessionRule{
		"SessRule1": &sessRule1,
		"SessRule2": &sessRule2,
	}
}
