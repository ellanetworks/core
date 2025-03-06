// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

func UeCmRegistration(ue *context.AmfUe, accessType models.AccessType, initialRegistrationInd bool) error {
	amfSelf := context.AMF_Self()
	guamiList := context.GetServedGuamiList()

	switch accessType {
	case models.AccessType__3_GPP_ACCESS:
		registrationData := models.Amf3GppAccessRegistration{
			AmfInstanceId:          amfSelf.NfId,
			InitialRegistrationInd: initialRegistrationInd,
			Guami: &models.Guami{
				PlmnId: &models.PlmnId{
					Mcc: guamiList[0].PlmnId.Mcc,
					Mnc: guamiList[0].PlmnId.Mnc,
				},
				AmfId: guamiList[0].AmfId,
			},
			RatType: ue.RatType,
			ImsVoPs: models.ImsVoPs_HOMOGENEOUS_NON_SUPPORT,
		}
		udm.EditRegistrationAmf3gppAccess(registrationData, ue.Supi)
	case models.AccessType_NON_3_GPP_ACCESS:
		return fmt.Errorf("Non-3GPP access is not supported")
	}
	return nil
}
