// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nssf

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

// Title in Problem Details for NSSF HTTP APIs
const (
	INVALID_REQUEST       = "Invalid request message framing"
	MALFORMED_REQUEST     = "Malformed request syntax"
	UNAUTHORIZED_CONSUMER = "Unauthorized NF service consumer"
	UNSUPPORTED_RESOURCE  = "Unsupported request resources"
)

// CheckStandardSnssai checks whether S-NSSAI is standard or non-standard value
// A standard S-NSSAI is only comprised of a standardized SST value and no SD
func CheckStandardSnssai(snssai models.Snssai) bool {
	if snssai.Sst >= 1 && snssai.Sst <= 3 && snssai.Sd == "" {
		return true
	}
	return false
}

// FindMappingWithServingSnssai finds target S-NSSAI mapping with serving S-NSSAIs from mapping of S-NSSAI(s)
func FindMappingWithServingSnssai(
	snssai models.Snssai, mappings []models.MappingOfSnssai,
) (models.MappingOfSnssai, bool) {
	for _, mapping := range mappings {
		if *mapping.ServingSnssai == snssai {
			return mapping, true
		}
	}
	return models.MappingOfSnssai{}, false
}

// AddAllowedSnssai finds target S-NSSAI mapping with home S-NSSAIs from mapping of S-NSSAI(s)
func FindMappingWithHomeSnssai(snssai models.Snssai, mappings []models.MappingOfSnssai) (models.MappingOfSnssai, bool) {
	for _, mapping := range mappings {
		if *mapping.HomeSnssai == snssai {
			return mapping, true
		}
	}
	return models.MappingOfSnssai{}, false
}

// AddAllowedSnssai: Add Allowed S-NSSAI to Authorized Network Slice Info
func AddAllowedSnssai(allowedSnssai models.AllowedSnssai, accessType models.AccessType,
	authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	hitAllowedNssai := false
	allowedNssaiNum := 8
	for i := range authorizedNetworkSliceInfo.AllowedNssaiList {
		if authorizedNetworkSliceInfo.AllowedNssaiList[i].AccessType == accessType {
			hitAllowedNssai = true
			if len(authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList) == allowedNssaiNum {
				logger.NssfLog.Infof("Unable to add a new Allowed S-NSSAI since already eight S-NSSAIs in Allowed NSSAI")
			} else {
				authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList = append(authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList, allowedSnssai)
			}
			break
		}
	}

	if !hitAllowedNssai {
		var allowedNssaiElement models.AllowedNssai
		allowedNssaiElement.AllowedSnssaiList = append(allowedNssaiElement.AllowedSnssaiList, allowedSnssai)
		allowedNssaiElement.AccessType = accessType

		authorizedNetworkSliceInfo.AllowedNssaiList = append(authorizedNetworkSliceInfo.AllowedNssaiList, allowedNssaiElement)
	}
}
