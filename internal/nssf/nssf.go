// Copyright 2024 Ella Networks

package nssf

import (
	"github.com/omec-project/openapi/models"
)

// GetNetworkSliceInfo returns the first Subscribed Snssai from the input parameter
func GetNetworkSliceInfo(param *models.SliceInfoForRegistration) *models.AuthorizedNetworkSliceInfo {
	return &models.AuthorizedNetworkSliceInfo{
		AllowedNssaiList: []models.AllowedNssai{
			{
				AllowedSnssaiList: []models.AllowedSnssai{
					{
						AllowedSnssai: param.SubscribedNssai[0].SubscribedSnssai,
					},
				},
				AccessType: models.AccessType__3_GPP_ACCESS,
			},
		},
	}
}
