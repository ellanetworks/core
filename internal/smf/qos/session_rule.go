// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/ellanetworks/core/internal/models"
)

// Handle Session Rule related info
type SessRulesUpdate struct {
	add            *models.SessionRule
	ActiveSessRule *models.SessionRule
}

func GetSessionRulesUpdate(pcfSessRule, ctxtSessRule *models.SessionRule) *SessRulesUpdate {
	change := SessRulesUpdate{}

	/// added rule
	if pcfSessRule != nil && ctxtSessRule == nil {
		change.add = pcfSessRule
		change.ActiveSessRule = pcfSessRule
	}

	// modified rule
	if pcfSessRule != nil && ctxtSessRule != nil {
		change.ActiveSessRule = pcfSessRule
	}

	return &change
}

func (polData *SmCtxtPolicyData) CommitSessionRulesUpdate(update *SessRulesUpdate) {
	if update.add != nil {
		polData.SmCtxtSessionRules.SessionRule = update.add
	}

	polData.SmCtxtSessionRules.ActiveRule = update.ActiveSessRule
}
