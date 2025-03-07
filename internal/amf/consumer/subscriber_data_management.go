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

func SDMGetAmData(ue *context.AmfUe) error {
	data, err := udm.GetAmDataAndSetAMSubscription(ue.Supi)
	if err != nil {
		return err
	}
	ue.AccessAndMobilitySubscriptionData = data
	return nil
}

func SDMGetSmfSelectData(ue *context.AmfUe) error {
	data, err := udm.GetAndSetSmfSelectData(ue.Supi)
	if err != nil {
		return err
	}
	ue.SmfSelectionData = data
	return nil
}

func SDMGetUeContextInSmfData(ue *context.AmfUe) (err error) {
	data, err := udm.GetUeContextInSmfData(ue.Supi)
	if err != nil {
		return err
	}
	ue.UeContextInSmfData = data
	return nil
}

func SDMSubscribe(ue *context.AmfUe) error {
	amfSelf := context.AMFSelf()
	sdmSubscription := &models.SdmSubscription{
		NfInstanceId: amfSelf.NfId,
		PlmnId: &models.PlmnId{
			Mcc: ue.PlmnId.Mcc,
			Mnc: ue.PlmnId.Mnc,
		},
	}
	err := udm.CreateSubscription(sdmSubscription, ue.Supi)
	if err != nil {
		return fmt.Errorf("subscription creation failed: %s", err.Error())
	}
	return nil
}

func SDMGetSliceSelectionSubscriptionData(ue *context.AmfUe) error {
	nssai, err := udm.GetNssai(ue.Supi)
	if err != nil {
		return fmt.Errorf("get nssai failed: %s", err.Error())
	}
	for _, defaultSnssai := range nssai.DefaultSingleNssais {
		subscribedSnssai := models.SubscribedSnssai{
			SubscribedSnssai: &models.Snssai{
				Sst: defaultSnssai.Sst,
				Sd:  defaultSnssai.Sd,
			},
			DefaultIndication: true,
		}
		ue.SubscribedNssai = append(ue.SubscribedNssai, subscribedSnssai)
	}
	for _, snssai := range nssai.SingleNssais {
		subscribedSnssai := models.SubscribedSnssai{
			SubscribedSnssai: &models.Snssai{
				Sst: snssai.Sst,
				Sd:  snssai.Sd,
			},
			DefaultIndication: false,
		}
		ue.SubscribedNssai = append(ue.SubscribedNssai, subscribedSnssai)
	}
	return nil
}
