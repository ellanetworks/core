// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/internal/models"
)

// InTaiList reports whether servedTai is present in taiList (PLMN + TAC match).
func InTaiList(servedTai models.Tai, taiList []models.Tai) bool {
	for _, tai := range taiList {
		if tai.Tac == servedTai.Tac && PlmnIDEqual(tai.PlmnID, servedTai.PlmnID) {
			return true
		}
	}

	return false
}

// PlmnIDEqual reports whether two PLMN identities are equal (MCC + MNC).
func PlmnIDEqual(a, b *models.PlmnID) bool {
	if a == b {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Mcc == b.Mcc && a.Mnc == b.Mnc
}

// AnyPLMNMatch reports whether any supported TAI broadcasts the operator PLMN.
func AnyPLMNMatch(supportedTAIs []SupportedTAI, operatorPLMN *models.PlmnID) bool {
	for _, tai := range supportedTAIs {
		if PlmnIDEqual(tai.Tai.PlmnID, operatorPLMN) {
			return true
		}
	}

	return false
}

// PlmnIDStringToModels parses a concatenated MCC+MNC string into a PlmnID.
func PlmnIDStringToModels(plmnIDStr string) models.PlmnID {
	if len(plmnIDStr) < 5 {
		return models.PlmnID{}
	}

	return models.PlmnID{
		Mcc: plmnIDStr[:3],
		Mnc: plmnIDStr[3:],
	}
}
