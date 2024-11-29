package qos_test

import "github.com/omec-project/openapi/models"

func TestMakeSamplePolicyDecision() *models.SmPolicyDecision {
	smPolDec := &models.SmPolicyDecision{
		PccRules:      TestMakePccRules(),
		SessRules:     TestMakeSessionRule(),
		QosDecs:       TestMakeQosData(),
		TraffContDecs: TestMakeTrafficControlData(),
	}

	return smPolDec
}

// TestMakePccRules - Locally generate PCC Rule data
func TestMakePccRules() map[string]*models.PccRule {
	pccRuleDef := models.PccRule{
		PccRuleId:  "255",
		Precedence: 255,
		RefQosData: []string{"QosData1"},
		RefTcData:  []string{"TC1"},
		FlowInfos:  make([]models.FlowInformation, 0),
	}

	flowInfosDef := []models.FlowInformation{
		{
			FlowDescription:   "permit out ip from any to assigned",
			PackFiltId:        "1",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
	}

	pccRuleDef.FlowInfos = append(pccRuleDef.FlowInfos, flowInfosDef...)

	pccRule1 := models.PccRule{
		PccRuleId:  "1",
		Precedence: 111,
		RefQosData: []string{"QosData1"},
		RefTcData:  []string{"TC1"},
		FlowInfos:  make([]models.FlowInformation, 0),
	}

	flowInfos := []models.FlowInformation{
		{
			FlowDescription:   "permit out ip from 1.1.1.1 1000-1200 to assigned",
			PackFiltId:        "1",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
		{
			FlowDescription:   "permit out 17 from 3.3.3.3/24 3000 to 4.4.4.4/24 4000",
			PackFiltId:        "2",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
	}

	pccRule1.FlowInfos = append(pccRule1.FlowInfos, flowInfos...)

	pccRule2 := models.PccRule{
		PccRuleId:  "2",
		Precedence: 222,
		RefQosData: []string{"QosData2"},
		RefTcData:  []string{"TC2"},
		FlowInfos:  make([]models.FlowInformation, 0),
	}

	flowInfos1 := []models.FlowInformation{
		{
			FlowDescription:   "permit out ip from 5.5.5.5 1000-1200 to assigned",
			PackFiltId:        "1",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
		{
			FlowDescription:   "permit out 17 from 3.3.3.3/24 3000 to 4.4.4.4/24 4000",
			PackFiltId:        "2",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
	}

	pccRule2.FlowInfos = append(pccRule2.FlowInfos, flowInfos1...)

	return map[string]*models.PccRule{"PccRule1": &pccRule1, "PccRule2": &pccRule2, "PccRuleDef": &pccRuleDef}
}

// TestMakeQosData - Locally generate Qos Flow data
func TestMakeQosData() map[string]*models.QosData {
	qosData1 := models.QosData{
		QosId:                "1",
		Var5qi:               9,
		MaxbrUl:              "101 Mbps",
		MaxbrDl:              "201 Mbps",
		GbrUl:                "11 Mbps",
		GbrDl:                "21 Mbps",
		PriorityLevel:        5,
		DefQosFlowIndication: true,
		Arp: &models.Arp{
			PriorityLevel: 3,
			PreemptCap:    models.PreemptionCapability_MAY_PREEMPT,
			PreemptVuln:   models.PreemptionVulnerability_PREEMPTABLE,
		},
	}

	qosData2 := models.QosData{
		QosId:                "2",
		Var5qi:               9,
		MaxbrUl:              "301 Mbps",
		MaxbrDl:              "401 Mbps",
		GbrUl:                "31 Mbps",
		GbrDl:                "41 Mbps",
		PriorityLevel:        3,
		DefQosFlowIndication: false,
		Arp: &models.Arp{
			PriorityLevel: 3,
			PreemptCap:    models.PreemptionCapability_NOT_PREEMPT,
			PreemptVuln:   models.PreemptionVulnerability_NOT_PREEMPTABLE,
		},
	}

	return map[string]*models.QosData{
		"QosData1": &qosData1,
		"QosData2": &qosData2,
	}
}

// TestMakeSessionRule - Locally generate Qos Flow data
func TestMakeSessionRule() map[string]*models.SessionRule {
	sessRule1 := models.SessionRule{
		SessRuleId: "RuleId-1",
		AuthSessAmbr: &models.Ambr{
			Uplink:   "77 Mbps",
			Downlink: "99 Mbps",
		},
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: 9,
			Arp: &models.Arp{
				PriorityLevel: 8,
				PreemptCap:    models.PreemptionCapability_MAY_PREEMPT,
				PreemptVuln:   models.PreemptionVulnerability_NOT_PREEMPTABLE,
			},
			PriorityLevel: 8,
		},
	}
	sessRule2 := models.SessionRule{
		SessRuleId: "RuleId-2",
		AuthSessAmbr: &models.Ambr{
			Uplink:   "55 Mbps",
			Downlink: "33 Mbps",
		},
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: 9,
			Arp: &models.Arp{
				PriorityLevel: 7,
				PreemptCap:    models.PreemptionCapability_MAY_PREEMPT,
				PreemptVuln:   models.PreemptionVulnerability_NOT_PREEMPTABLE,
			},
			PriorityLevel: 7,
		},
	}

	return map[string]*models.SessionRule{
		"SessRule1": &sessRule1,
		"SessRule2": &sessRule2,
	}
}

// TestMakeTrafficControlData - Locally generate Traffic Control data
func TestMakeTrafficControlData() map[string]*models.TrafficControlData {
	tc1 := models.TrafficControlData{
		TcId:       "TC1",
		FlowStatus: models.FlowStatus_ENABLED,
	}

	tc2 := models.TrafficControlData{
		TcId:       "TC2",
		FlowStatus: models.FlowStatus_ENABLED,
	}

	return map[string]*models.TrafficControlData{"TC1": &tc1, "TC2": &tc2}
}
