// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/ellanetworks/core/internal/models"
)

// Handle Session Rule related info
type SessRulesUpdate struct {
	add, mod, del  *models.SessionRule
	ActiveSessRule *models.SessionRule
	activeRuleName uint8
}

// Get Session rule changes delta
func GetSessionRulesUpdate(pcfSessRule, ctxtSessRule *models.SessionRule) *SessRulesUpdate {
	change := SessRulesUpdate{}

	// deleted rule
	if pcfSessRule == nil && ctxtSessRule != nil {
		change.del = ctxtSessRule
	}

	/// added rule
	if pcfSessRule != nil && ctxtSessRule == nil {
		change.add = pcfSessRule
		change.ActiveSessRule = pcfSessRule
	}

	// modified rule
	if pcfSessRule != nil && ctxtSessRule != nil {
		change.mod = pcfSessRule
		change.ActiveSessRule = pcfSessRule
	}

	return &change
}

func CommitSessionRulesUpdate(smCtxtPolData *SmCtxtPolicyData, update *SessRulesUpdate) {
	// Add new Rule
	if update.add != nil {
		smCtxtPolData.SmCtxtSessionRules.SessionRule = update.add
	}

	// Delete Rule
	if update.del != nil {
		smCtxtPolData.SmCtxtSessionRules.SessionRule = nil
	}

	// Set Active Rule
	smCtxtPolData.SmCtxtSessionRules.ActiveRule = update.ActiveSessRule
	smCtxtPolData.SmCtxtSessionRules.ActiveRuleName = update.activeRuleName
}
