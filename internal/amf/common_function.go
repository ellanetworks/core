// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func InTaiList(servedTai models.Tai, taiList []models.Tai) bool {
	for _, tai := range taiList {
		if tai.Tac == servedTai.Tac && plmnIDEqual(tai.PlmnID, servedTai.PlmnID) {
			return true
		}
	}

	return false
}

func plmnIDEqual(a, b *models.PlmnID) bool {
	if a == b {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Mcc == b.Mcc && a.Mnc == b.Mnc
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
	targetUe.SourceUe = sourceUe
	sourceUe.TargetUe = targetUe

	return nil
}

func DetachSourceUeTargetUe(ranUe *RanUe) {
	if ranUe == nil {
		logger.AmfLog.Error("ranUe is Nil")
		return
	}

	if ranUe.TargetUe != nil {
		targetUe := ranUe.TargetUe

		ranUe.TargetUe = nil
		targetUe.SourceUe = nil
	} else if ranUe.SourceUe != nil {
		source := ranUe.SourceUe

		ranUe.SourceUe = nil
		source.TargetUe = nil
	}
}
