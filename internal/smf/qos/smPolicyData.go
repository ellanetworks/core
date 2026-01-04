// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/ellanetworks/core/internal/models"
)

type PolicyUpdate struct {
	SessRuleUpdate *models.SessionRule
	QosFlowUpdate  *models.QosData
}

func BuildSmPolicyUpdate(smPolicyDecision *models.SmPolicyDecision) *PolicyUpdate {
	update := &PolicyUpdate{
		QosFlowUpdate:  smPolicyDecision.QosDecs,
		SessRuleUpdate: smPolicyDecision.SessRule,
	}

	update.QosFlowUpdate.QFI = DefaultQFI

	return update
}
