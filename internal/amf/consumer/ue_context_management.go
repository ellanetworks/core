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
			AmfInstanceID:          amfSelf.NfId,
			InitialRegistrationInd: initialRegistrationInd,
			Guami: &models.Guami{
				PlmnID: &models.PlmnID{
					Mcc: guamiList[0].PlmnID.Mcc,
					Mnc: guamiList[0].PlmnID.Mnc,
				},
				AmfID: guamiList[0].AmfID,
			},
			RatType: ue.RatType,
			ImsVoPs: models.ImsVoPs_HOMOGENEOUS_NON_SUPPORT,
		}
		err := udm.EditRegistrationAmf3gppAccess(registrationData, ue.Supi)
		if err != nil {
			return err
		}
	case models.AccessType_NON_3_GPP_ACCESS:
		return fmt.Errorf("Non-3GPP access is not supported")
	}
	return nil
}
