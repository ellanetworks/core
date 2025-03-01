// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func QosDataString(q *models.QosData) string {
	if q == nil {
		return ""
	}
	return fmt.Sprintf("QosData:[QosId:[%v], Var5QI:[%v], MaxBrUl:[%v], MaxBrDl:[%v], GBrUl:[%v], GBrDl:[%v], PriorityLevel:[%v], ARP:[%v], DQFI:[%v]]",
		q.QosId, q.Var5qi, q.MaxbrUl, q.MaxbrDl, q.GbrUl, q.GbrDl, q.PriorityLevel, q.Arp, q.DefQosFlowIndication)
}

func SessRuleString(s *models.SessionRule) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("SessRule:[RuleId:[%v], Ambr:[Dl:[%v], Ul:[%v]], AuthDefQos:[Var5QI:[%v], PriorityLevel:[%v], ARP:[%v]]]",
		s.SessRuleId, s.AuthSessAmbr.Downlink, s.AuthSessAmbr.Uplink, s.AuthDefQos.Var5qi, s.AuthDefQos.PriorityLevel, s.AuthDefQos.Arp)
}

func PccRuleString(pcc *models.PccRule) string {
	if pcc == nil {
		return ""
	}
	logger.SmfLog.Warnf("PccRuleString: %v", pcc)
	logger.SmfLog.Warnf("PccRuleString RefQosData: %v", pcc.RefQosData)

	return fmt.Sprintf("PccRule:[RuleId:[%v], Precdence:[%v], RefQosData:[%v], flow:[%v]]",
		pcc.PccRuleId, pcc.Precedence, pcc.RefQosData[0], PccFlowInfosString(pcc.FlowInfos))
}

func TCDataString(tcData *models.TrafficControlData) string {
	return fmt.Sprintf("TC Data:[Id:[%v], FlowStatus:[%v]]", tcData.TcId, tcData.FlowStatus)
}

func PccFlowInfosString(flows []models.FlowInformation) []string {
	var flowStrs []string
	for _, flow := range flows {
		str := fmt.Sprintf("\nFlowInfo:[flowDesc:[%v], PFId:[%v], direction:[%v]]",
			flow.FlowDescription, flow.PackFiltId, flow.FlowDirection)

		flowStrs = append(flowStrs, str)
	}
	return flowStrs
}

func (obj PolicyUpdate) String() string {
	return fmt.Sprintf("Policy Update:[\nPccRule:[%v], \nSessRules:[%v], \nQosData:[%v], \nTcData:[%v]]",
		obj.PccRuleUpdate, obj.SessRuleUpdate, obj.QosFlowUpdate, obj.TCUpdate)
}

func (obj PccRulesUpdate) String() string {
	str := "\nPCC Rule Changes:"

	// To be added
	strAdd := ""
	for name, rule := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, PccRuleString(rule))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, rule := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, PccRuleString(rule))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, rule := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, PccRuleString(rule))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

func (obj SessRulesUpdate) String() string {
	str := "\nSess Rule Changes:"

	// To be added
	strAdd := ""
	for name, rule := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, SessRuleString(rule))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, rule := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, SessRuleString(rule))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, rule := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, SessRuleString(rule))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

func (obj QosFlowsUpdate) String() string {
	str := "\nQos Data Changes:"

	// To be added
	strAdd := ""
	for name, val := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, QosDataString(val))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, val := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, QosDataString(val))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, val := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, QosDataString(val))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

func (obj TrafficControlUpdate) String() string {
	str := "\nTC Data Changes:"

	// To be added
	strAdd := ""
	for name, val := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, TCDataString(val))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, val := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, TCDataString(val))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, val := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, TCDataString(val))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}
