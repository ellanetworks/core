// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func buildAmPolicyReqTriggers(triggers []models.RequestTrigger) (amPolicyReqTriggers []models.AmPolicyReqTrigger) {
	for _, trigger := range triggers {
		switch trigger {
		case models.RequestTriggerLocCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerLocationChange)
		case models.RequestTriggerPraCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerPraChange)
		case models.RequestTriggerServAreaCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerSariChange)
		case models.RequestTriggerRfspCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerRfspIndexChange)
		}
	}
	return
}

// This operation is called "RegistrationCompleteNotify" at TS 23.502
func RegistrationStatusUpdate(ue *context.AmfUe, request models.UeRegStatusUpdateReqData) (
	bool, error,
) {
	logger.AmfLog.Warn("UE registration status update is not implemented")
	return false, nil
}
