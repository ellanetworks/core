// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
)

func QosDataString(q *models.QosData) string {
	if q == nil {
		return ""
	}
	return fmt.Sprintf("QosData:[QFI:[%v], Var5QI:[%v], MaxBrUl:[%v], MaxBrDl:[%v], GBrUl:[%v], GBrDl:[%v], PriorityLevel:[%v], ARP:[%v], DQFI:[%v]]",
		q.QFI, q.Var5qi, q.MaxbrUl, q.MaxbrDl, q.GbrUl, q.GbrDl, q.PriorityLevel, q.Arp, q.DefQosFlowIndication)
}

func SessRuleString(s *models.SessionRule) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("SessRule:Ambr:[Dl:[%v], Ul:[%v]], AuthDefQos:[Var5QI:[%v], PriorityLevel:[%v], ARP:[%v]]]",
		s.AuthSessAmbr.Downlink, s.AuthSessAmbr.Uplink, s.AuthDefQos.Var5qi, s.AuthDefQos.PriorityLevel, s.AuthDefQos.Arp)
}

func (obj PolicyUpdate) String() string {
	return fmt.Sprintf("Policy Update:\nSessRules:[%v], \nQosData:[%v]]",
		obj.SessRuleUpdate, obj.QosFlowUpdate)
}

func (obj SessRulesUpdate) String() string {
	str := "\nSess Rule Changes:"

	// To be added
	strAdd := ""
	if obj.add != nil {
		strAdd += fmt.Sprintf("\n%v", SessRuleString(obj.add))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	if obj.mod != nil {
		strMod += fmt.Sprintf("\n%v", SessRuleString(obj.mod))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	if obj.del != nil {
		strDel += fmt.Sprintf("\n%v", SessRuleString(obj.del))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

func (upd QosFlowsUpdate) String() string {
	str := "\nQos Data Changes:"

	// To be added
	strAdd := ""
	if upd.Add != nil {
		strAdd += fmt.Sprintf("\n%v", QosDataString(upd.Add))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	if upd.mod != nil {
		strMod += fmt.Sprintf("\n%v", QosDataString(upd.mod))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	if upd.del != nil {
		strDel += fmt.Sprintf("\n%v", QosDataString(upd.del))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}
