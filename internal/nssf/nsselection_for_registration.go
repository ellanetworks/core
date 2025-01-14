// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nssf

import (
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

// Set Allowed NSSAI with Subscribed S-NSSAI(s) which are marked as default S-NSSAI(s)
func useDefaultSubscribedSnssai(
	param NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	var mappingOfSnssai []models.MappingOfSnssai
	if param.HomePlmnID != nil {
		// Find mapping of Subscribed S-NSSAI of UE's HPLMN to S-NSSAI in Serving PLMN from NSSF configuration
		logger.NssfLog.Warnf("No S-NSSAI mapping of UE's HPLMN %+v in NSSF configuration", *param.HomePlmnID)
		return
	}

	for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
		if subscribedSnssai.DefaultIndication {
			// Subscribed S-NSSAI is marked as default S-NSSAI

			var mappingOfSubscribedSnssai models.Snssai
			if param.HomePlmnID != nil && !CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
				targetMapping, found := FindMappingWithHomeSnssai(*subscribedSnssai.SubscribedSnssai, mappingOfSnssai)

				if !found {
					logger.NssfLog.Warnf("No mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
						*subscribedSnssai.SubscribedSnssai,
						*param.HomePlmnID)
					continue
				} else {
					mappingOfSubscribedSnssai = *targetMapping.ServingSnssai
				}
			} else {
				mappingOfSubscribedSnssai = *subscribedSnssai.SubscribedSnssai
			}

			if param.Tai != nil {
				continue
			}

			var allowedSnssaiElement models.AllowedSnssai
			allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
			*allowedSnssaiElement.AllowedSnssai = mappingOfSubscribedSnssai

			if param.HomePlmnID != nil {
				allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
				*allowedSnssaiElement.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai
			}

			accessType := models.AccessType__3_GPP_ACCESS

			AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)
		}
	}
}

// Set Configured NSSAI with S-NSSAI(s) in Requested NSSAI which are marked as Default Configured NSSAI
func useDefaultConfiguredNssai(
	param NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	for _, requestedSnssai := range param.SliceInfoRequestForRegistration.RequestedNssai {
		// Check whether the Default Configured S-NSSAI is standard, which could be commonly decided by all roaming partners
		if !CheckStandardSnssai(requestedSnssai) {
			logger.NssfLog.Infof("S-NSSAI %+v in Requested NSSAI which based on Default Configured NSSAI is not standard",
				requestedSnssai)
			continue
		}

		// Check whether the Default Configured S-NSSAI is subscribed
		for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
			if requestedSnssai == *subscribedSnssai.SubscribedSnssai {
				var configuredSnssai models.ConfiguredSnssai
				configuredSnssai.ConfiguredSnssai = new(models.Snssai)
				*configuredSnssai.ConfiguredSnssai = requestedSnssai

				authorizedNetworkSliceInfo.ConfiguredNssai = append(authorizedNetworkSliceInfo.ConfiguredNssai, configuredSnssai)
				break
			}
		}
	}
}

// Network slice selection for registration
// The function is executed when the IE, `slice-info-request-for-registration`, is provided in query parameters
func nsselectionForRegistration(param NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo, problemDetails *models.ProblemDetails) error {
	if param.HomePlmnID != nil {
		// Check whether UE's Home PLMN is supported when UE is a roamer
		authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, param.SliceInfoRequestForRegistration.RequestedNssai...)
		return nil
	}

	if param.Tai != nil {
		// Check whether UE's current TA is supported when UE provides TAI
		authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, param.SliceInfoRequestForRegistration.RequestedNssai...)
		return nil
	}

	if param.SliceInfoRequestForRegistration.RequestMapping {
		// Based on TS 29.531 v15.2.0, when `requestMapping` is set to true, the NSSF shall return the VPLMN specific
		// mapped S-NSSAI values for the S-NSSAI values in `subscribedNssai`. But also `sNssaiForMapping` shall be
		// provided if `requestMapping` is set to true. In the implementation, the NSSF would return mapped S-NSSAIs
		// for S-NSSAIs in both `sNssaiForMapping` and `subscribedSnssai` if present

		if param.HomePlmnID == nil {
			problemDetail := "[Query Parameter] `home-plmn-id` should be provided when requesting VPLMN specific mapped S-NSSAI values"
			*problemDetails = models.ProblemDetails{
				Title:  INVALID_REQUEST,
				Status: http.StatusBadRequest,
				Detail: problemDetail,
				InvalidParams: []models.InvalidParam{
					{
						Param:  "home-plmn-id",
						Reason: problemDetail,
					},
				},
			}

			return fmt.Errorf("bad request")
		}

		logger.NssfLog.Warnf("No S-NSSAI mapping of UE's HPLMN %+v in NSSF configuration", *param.HomePlmnID)

		return nil
	}

	if len(param.SliceInfoRequestForRegistration.RequestedNssai) > 0 {
		// Requested NSSAI is provided
		// Verify which S-NSSAI(s) in the Requested NSSAI are permitted based on comparing the Subscribed S-NSSAI(s)

		// Check if any Requested S-NSSAIs is present in Subscribed S-NSSAIs
		checkIfRequestAllowed := false

		for _, requestedSnssai := range param.SliceInfoRequestForRegistration.RequestedNssai {
			if param.Tai != nil {
				// Requested S-NSSAI does not supported in UE's current TA
				// Add it to Rejected NSSAI in TA
				authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, requestedSnssai)
				continue
			}

			var mappingOfRequestedSnssai models.Snssai
			if param.HomePlmnID != nil && !CheckStandardSnssai(requestedSnssai) {
				// Standard S-NSSAIs are supported to be commonly decided by all roaming partners
				// Only non-standard S-NSSAIs are required to find mappings
				targetMapping, found := FindMappingWithServingSnssai(requestedSnssai,
					param.SliceInfoRequestForRegistration.MappingOfNssai)

				if !found {
					authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, requestedSnssai)
					continue
				} else {
					mappingOfRequestedSnssai = *targetMapping.HomeSnssai
				}
			} else {
				mappingOfRequestedSnssai = requestedSnssai
			}

			hitSubscription := false
			for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
				if mappingOfRequestedSnssai == *subscribedSnssai.SubscribedSnssai {
					// Requested S-NSSAI matches one of Subscribed S-NSSAI
					// Add it to Allowed NSSAI list
					hitSubscription = true

					var allowedSnssaiElement models.AllowedSnssai
					allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
					*allowedSnssaiElement.AllowedSnssai = requestedSnssai
					if param.HomePlmnID != nil && !CheckStandardSnssai(requestedSnssai) {
						allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
						*allowedSnssaiElement.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai
					}

					accessType := models.AccessType__3_GPP_ACCESS

					AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)

					checkIfRequestAllowed = true
					break
				}
			}

			if !hitSubscription {
				// Requested S-NSSAI does not match any Subscribed S-NSSAI
				// Add it to Rejected NSSAI in PLMN
				authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, requestedSnssai)
			}
		}

		if !checkIfRequestAllowed {
			// No S-NSSAI from Requested NSSAI is present in Subscribed S-NSSAIs
			// Subscribed S-NSSAIs marked as default are used
			useDefaultSubscribedSnssai(param, authorizedNetworkSliceInfo)
		}
	} else {
		// No Requested NSSAI is provided
		// Subscribed S-NSSAIs marked as default are used
		useDefaultSubscribedSnssai(param, authorizedNetworkSliceInfo)
	}

	if param.SliceInfoRequestForRegistration.DefaultConfiguredSnssaiInd {
		// Default Configured NSSAI Indication is received from AMF
		// Determine the Configured NSSAI based on the Default Configured NSSAI
		useDefaultConfiguredNssai(param, authorizedNetworkSliceInfo)
	}

	return nil
}
