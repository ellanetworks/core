// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
)

func InTaiList(servedTai models.Tai, taiList []models.Tai) bool {
	for _, tai := range taiList {
		if tai.Tac == servedTai.Tac && PlmnIDEqual(tai.PlmnID, servedTai.PlmnID) {
			return true
		}
	}

	return false
}

func PlmnIDEqual(a, b *models.PlmnID) bool {
	if a == b {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Mcc == b.Mcc && a.Mnc == b.Mnc
}

func AnyPLMNMatch(supportedTAIs []SupportedTAI, operatorPLMN *models.PlmnID) bool {
	for _, tai := range supportedTAIs {
		if PlmnIDEqual(tai.Tai.PlmnID, operatorPLMN) {
			return true
		}
	}

	return false
}

func AttachSourceUeTargetUe(sourceUe, targetUe *RanUe) error {
	if sourceUe == nil {
		return fmt.Errorf("source ue is nil")
	}

	if targetUe == nil {
		return fmt.Errorf("target ue is nil")
	}

	amfUe := sourceUe.amfUe
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	targetUe.amfUe = amfUe

	amfUe.BeginHandover(sourceUe, targetUe)

	return nil
}
