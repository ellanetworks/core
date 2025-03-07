// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func CompareUserLocation(loc1 models.UserLocation, loc2 models.UserLocation) bool {
	if loc1.EutraLocation != nil && loc2.EutraLocation != nil {
		eutraloc1 := *loc1.EutraLocation
		eutraloc2 := *loc2.EutraLocation
		eutraloc1.UeLocationTimestamp = nil
		eutraloc2.UeLocationTimestamp = nil
		return reflect.DeepEqual(eutraloc1, eutraloc2)
	}
	if loc1.N3gaLocation != nil && loc2.N3gaLocation != nil {
		return reflect.DeepEqual(loc1, loc2)
	}
	if loc1.NrLocation != nil && loc2.NrLocation != nil {
		nrloc1 := *loc1.NrLocation
		nrloc2 := *loc2.NrLocation
		nrloc1.UeLocationTimestamp = nil
		nrloc2.UeLocationTimestamp = nil
		return reflect.DeepEqual(nrloc1, nrloc2)
	}

	return false
}

func InTaiList(servedTai models.Tai, taiList []models.Tai) bool {
	for _, tai := range taiList {
		if reflect.DeepEqual(tai, servedTai) {
			return true
		}
	}
	return false
}

func TacInAreas(targetTac string, areas []models.Area) bool {
	for _, area := range areas {
		for _, tac := range area.Tacs {
			if targetTac == tac {
				return true
			}
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
	amfUe := sourceUe.AmfUe
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}
	targetUe.AmfUe = amfUe
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
