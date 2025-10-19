// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/ellanetworks/core/internal/models"
)

// Define SMF Session-Rule/PccRule/Rule-Qos-Data
type PolicyUpdate struct {
	SessRuleUpdate *SessRulesUpdate
	QosFlowUpdate  *QosFlowsUpdate

	// relevant SM Policy Decision from PCF
	SmPolicyDecision *models.SmPolicyDecision
}

type SmCtxtPolicyData struct {
	// maintain all session rule-info and current active sess rule
	SmCtxtQosData      SmCtxtQosData
	SmCtxtSessionRules SmCtxtSessionRulesInfo
}

// maintain all session rule-info and current active sess rule
type SmCtxtSessionRulesInfo struct {
	ActiveRule     *models.SessionRule
	SessionRule    *models.SessionRule
	ActiveRuleName uint8
}

type SmCtxtQosData struct {
	QosData *models.QosData
}

func BuildSmPolicyUpdate(smCtxtPolData *SmCtxtPolicyData, smPolicyDecision *models.SmPolicyDecision) *PolicyUpdate {
	update := &PolicyUpdate{}

	// Keep copy of SmPolicyDecision received from PCF
	update.SmPolicyDecision = smPolicyDecision

	// Qos Flows update
	update.QosFlowUpdate = GetQosFlowDescUpdate(smPolicyDecision.QosDecs, smCtxtPolData.SmCtxtQosData.QosData)

	// Session Rule update
	update.SessRuleUpdate = GetSessionRulesUpdate(smPolicyDecision.SessRule, smCtxtPolData.SmCtxtSessionRules.SessionRule)

	return update
}

func CommitSmPolicyDecision(smCtxtPolData *SmCtxtPolicyData, smPolicyUpdate *PolicyUpdate) error {
	// Update Qos Flows
	if smPolicyUpdate.QosFlowUpdate != nil {
		CommitQosFlowDescUpdate(smCtxtPolData, smPolicyUpdate.QosFlowUpdate)
	}

	// Update Session Rules
	if smPolicyUpdate.SessRuleUpdate != nil {
		CommitSessionRulesUpdate(smCtxtPolData, smPolicyUpdate.SessRuleUpdate)
	}

	return nil
}
