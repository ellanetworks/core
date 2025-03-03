// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
)

func NSSelectionGetForRegistration(ue *context.AmfUe, requestedNssai []models.MappingOfSnssai) error {
	res := &models.AuthorizedNetworkSliceInfo{
		AllowedNssaiList: []models.AllowedNssai{
			{
				AllowedSnssaiList: []models.AllowedSnssai{
					{
						AllowedSnssai: ue.SubscribedNssai[0].SubscribedSnssai,
					},
				},
				AccessType: models.AccessType__3_GPP_ACCESS,
			},
		},
	}
	ue.NetworkSliceInfo = res
	for _, allowedNssai := range res.AllowedNssaiList {
		ue.AllowedNssai[allowedNssai.AccessType] = allowedNssai.AllowedSnssaiList
	}
	return nil
}
