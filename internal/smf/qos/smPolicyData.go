// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/ellanetworks/core/internal/models"
)

type PolicyUpdate struct {
	SessRuleUpdate *SessRulesUpdate
	QosFlowUpdate  *QosFlowsUpdate
}

type SmCtxtPolicyData struct {
	SmCtxtQosData      *models.QosData
	SmCtxtSessionRules SmCtxtSessionRulesInfo
}

type SmCtxtSessionRulesInfo struct {
	ActiveRule  *models.SessionRule
	SessionRule *models.SessionRule
}

func BuildSmPolicyUpdate(smCtxtPolData *SmCtxtPolicyData, smPolicyDecision *models.SmPolicyDecision) *PolicyUpdate {
	update := &PolicyUpdate{}

	update.QosFlowUpdate = GetQosFlowDescUpdate(smPolicyDecision.QosDecs, smCtxtPolData.SmCtxtQosData)

	update.SessRuleUpdate = GetSessionRulesUpdate(smPolicyDecision.SessRule, smCtxtPolData.SmCtxtSessionRules.SessionRule)

	return update
}

func (polData *SmCtxtPolicyData) CommitSmPolicyDecision(smPolicyUpdate *PolicyUpdate) error {
	if smPolicyUpdate.QosFlowUpdate != nil {
		polData.CommitQosFlowDescUpdate(smPolicyUpdate.QosFlowUpdate)
	}

	if smPolicyUpdate.SessRuleUpdate != nil {
		polData.CommitSessionRulesUpdate(smPolicyUpdate.SessRuleUpdate)
	}

	return nil
}
