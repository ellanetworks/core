// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
)

func UeCmRegistration(ue *context.AmfUe, accessType models.AccessType, initialRegistrationInd bool) (
	*models.ProblemDetails, error,
) {
	amfSelf := context.AMF_Self()
	guamiList := context.GetServedGuamiList()

	switch accessType {
	case models.AccessType__3_GPP_ACCESS:
		registrationData := models.Amf3GppAccessRegistration{
			AmfInstanceId:          amfSelf.NfId,
			InitialRegistrationInd: initialRegistrationInd,
			Guami:                  &guamiList[0],
			RatType:                ue.RatType,
			ImsVoPs:                models.ImsVoPs_HOMOGENEOUS_NON_SUPPORT,
		}
		err := udm.EditRegistrationAmf3gppAccess(registrationData, ue.Supi)
		if err != nil {
			return nil, err
		}
	case models.AccessType_NON_3_GPP_ACCESS:
		// log an error
		return nil, openapi.ReportError("Non-3GPP access is not supported")
	}

	return nil, nil
}
