// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/omec-project/openapi/models"
)

type PccRulesUpdate struct {
	add, mod, del map[string]*models.PccRule
}

func GetPccRulesUpdate(pcfPccRules, ctxtPccRules map[string]*models.PccRule) *PccRulesUpdate {
	if len(pcfPccRules) == 0 {
		return nil
	}

	change := PccRulesUpdate{
		add: make(map[string]*models.PccRule),
		mod: make(map[string]*models.PccRule),
		del: make(map[string]*models.PccRule),
	}

	// Compare against Ctxt rules to get added or modified rules
	for name, pcfRule := range pcfPccRules {
		// if pcfRule is nil then it need to be deleted
		if pcfRule == nil {
			change.del[name] = pcfRule // nil
			continue
		}

		// match against SM ctxt Rules for add/mod
		if ctxtrule := ctxtPccRules[name]; ctxtrule == nil {
			change.add[name] = pcfRule
		} else if GetPccRuleChanges(pcfRule, ctxtrule) {
			change.mod[name] = pcfRule
		}
	}

	return &change
}

func CommitPccRulesUpdate(smCtxtPolData *SmCtxtPolicyData, update *PccRulesUpdate) {
	// Iterate through Add/Mod/Del rules

	// Add new Rules
	if len(update.add) > 0 {
		for name, rule := range update.add {
			smCtxtPolData.SmCtxtPccRules.PccRules[name] = rule
		}
	}

	// Del Rules
	if len(update.del) > 0 {
		for name := range update.del {
			delete(smCtxtPolData.SmCtxtPccRules.PccRules, name)
		}
	}
}

// Get the difference between 2 pcc rules
func GetPccRuleChanges(s, d *models.PccRule) bool {
	return false
}

func (upd *PccRulesUpdate) GetAddPccRuleUpdate() map[string]*models.PccRule {
	return upd.add
}
