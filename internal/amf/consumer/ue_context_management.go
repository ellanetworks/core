// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

func UeCmRegistration(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, initialRegistrationInd bool) error {
	amfSelf := context.AMFSelf()
	guamiList := context.GetServedGuamiList(ctx)

	switch accessType {
	case models.AccessType3GPPAccess:
		registrationData := models.Amf3GppAccessRegistration{
			AmfInstanceID:          amfSelf.NfID,
			InitialRegistrationInd: initialRegistrationInd,
			Guami: &models.Guami{
				PlmnID: &models.PlmnID{
					Mcc: guamiList[0].PlmnID.Mcc,
					Mnc: guamiList[0].PlmnID.Mnc,
				},
				AmfID: guamiList[0].AmfID,
			},
			RatType: ue.RatType,
			ImsVoPs: models.ImsVoPsHomogeneousNonSupport,
		}
		err := udm.EditRegistrationAmf3gppAccess(ctx, registrationData, ue.Supi)
		if err != nil {
			return err
		}
	case models.AccessTypeNon3GPPAccess:
		return fmt.Errorf("Non-3GPP access is not supported")
	}
	return nil
}
