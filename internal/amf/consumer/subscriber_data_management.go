// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"github.com/ellanetworks/core/internal/amf/context"
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/omec-project/openapi/models"
)

func SDMGetAmData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	data, err := udm.GetAmDataAndSetAMSubscription(ue.Supi)
	if err != nil {
		return nil, err
	}
	ue.AccessAndMobilitySubscriptionData = data
	return nil, nil
}

func SDMGetSmfSelectData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	data, err := udm.GetAndSetSmfSelectData(ue.Supi)
	if err != nil {
		return nil, err
	}
	ue.SmfSelectionData = data
	return nil, nil
}

func SDMGetUeContextInSmfData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	data, err := udm.GetUeContextInSmfData(ue.Supi)
	if err != nil {
		return nil, err
	}
	ue.UeContextInSmfData = data
	return nil, nil
}

func SDMSubscribe(ue *context.AmfUe) (*models.ProblemDetails, error) {
	amfSelf := context.AMF_Self()
	sdmSubscription := &coreModels.SdmSubscription{
		NfInstanceId: amfSelf.NfId,
		PlmnId: &coreModels.PlmnId{
			Mcc: ue.PlmnId.Mcc,
			Mnc: ue.PlmnId.Mnc,
		},
	}

	err := udm.CreateSubscription(sdmSubscription, ue.Supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return problemDetails, nil
	}
	return nil, nil
}

func SDMGetSliceSelectionSubscriptionData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	nssai, err := udm.GetNssai(ue.Supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return problemDetails, nil
	}
	for _, defaultSnssai := range nssai.DefaultSingleNssais {
		subscribedSnssai := coreModels.SubscribedSnssai{
			SubscribedSnssai: &coreModels.Snssai{
				Sst: defaultSnssai.Sst,
				Sd:  defaultSnssai.Sd,
			},
			DefaultIndication: true,
		}
		ue.SubscribedNssai = append(ue.SubscribedNssai, subscribedSnssai)
	}
	for _, snssai := range nssai.SingleNssais {
		subscribedSnssai := coreModels.SubscribedSnssai{
			SubscribedSnssai: &coreModels.Snssai{
				Sst: snssai.Sst,
				Sd:  snssai.Sd,
			},
			DefaultIndication: false,
		}
		ue.SubscribedNssai = append(ue.SubscribedNssai, subscribedSnssai)
	}
	return nil, nil
}
