/*
 * NSSF NS Selection
 *
 * NSSF Network Slice Selection Service
 */

package producer

import (
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/nssf/logger"
	"github.com/yeastengine/ella/internal/nssf/plugin"
	"github.com/yeastengine/ella/internal/nssf/util"
)

// Set Allowed NSSAI with Subscribed S-NSSAI(s) which are marked as default S-NSSAI(s)
func useDefaultSubscribedSnssai(
	param plugin.NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	var mappingOfSnssai []models.MappingOfSnssai
	if param.HomePlmnId != nil {
		// Find mapping of Subscribed S-NSSAI of UE's HPLMN to S-NSSAI in Serving PLMN from NSSF configuration
		mappingOfSnssai = util.GetMappingOfPlmnFromConfig(*param.HomePlmnId)

		if mappingOfSnssai == nil {
			logger.Nsselection.Warnf("No S-NSSAI mapping of UE's HPLMN %+v in NSSF configuration", *param.HomePlmnId)
			return
		}
	}

	for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
		if subscribedSnssai.DefaultIndication {
			// Subscribed S-NSSAI is marked as default S-NSSAI

			var mappingOfSubscribedSnssai models.Snssai
			// TODO: Compared with Restricted S-NSSAI list in configuration under roaming scenario
			if param.HomePlmnId != nil && !util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
				targetMapping, found := util.FindMappingWithHomeSnssai(*subscribedSnssai.SubscribedSnssai, mappingOfSnssai)

				if !found {
					logger.Nsselection.Warnf("No mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
						*subscribedSnssai.SubscribedSnssai,
						*param.HomePlmnId)
					continue
				} else {
					mappingOfSubscribedSnssai = *targetMapping.ServingSnssai
				}
			} else {
				mappingOfSubscribedSnssai = *subscribedSnssai.SubscribedSnssai
			}

			if param.Tai != nil && !util.CheckSupportedSnssaiInTa(mappingOfSubscribedSnssai, *param.Tai) {
				continue
			}

			var allowedSnssaiElement models.AllowedSnssai
			allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
			*allowedSnssaiElement.AllowedSnssai = mappingOfSubscribedSnssai

			if param.HomePlmnId != nil && !util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
				allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
				*allowedSnssaiElement.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai
			}

			// Default Access Type is set to 3GPP Access if no TAI is provided
			// TODO: Depend on operator implementation, it may also return S-NSSAIs in all valid Access Type if
			//       UE's Access Type could not be identified
			var accessType models.AccessType = models.AccessType__3_GPP_ACCESS
			if param.Tai != nil {
				accessType = util.GetAccessTypeFromConfig(*param.Tai)
			}

			util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)
		}
	}
}

// Set Configured NSSAI with S-NSSAI(s) in Requested NSSAI which are marked as Default Configured NSSAI
func useDefaultConfiguredNssai(
	param plugin.NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	for _, requestedSnssai := range param.SliceInfoRequestForRegistration.RequestedNssai {
		// Check whether the Default Configured S-NSSAI is standard, which could be commonly decided by all roaming partners
		if !util.CheckStandardSnssai(requestedSnssai) {
			logger.Nsselection.Infof("S-NSSAI %+v in Requested NSSAI which based on Default Configured NSSAI is not standard",
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
func nsselectionForRegistration(param plugin.NsselectionQueryParameter,
	authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
	problemDetails *models.ProblemDetails,
) int {
	var status int
	if param.HomePlmnId != nil {
		// Check whether UE's Home PLMN is supported when UE is a roamer
		if !util.CheckSupportedHplmn(*param.HomePlmnId) {
			authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, param.SliceInfoRequestForRegistration.RequestedNssai...)

			status = http.StatusOK
			return status
		}
	}

	if param.Tai != nil {
		// Check whether UE's current TA is supported when UE provides TAI
		if !util.CheckSupportedTa(*param.Tai) {
			authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, param.SliceInfoRequestForRegistration.RequestedNssai...)

			status = http.StatusOK
			return status
		}
	}

	if param.SliceInfoRequestForRegistration.RequestMapping {
		// Based on TS 29.531 v15.2.0, when `requestMapping` is set to true, the NSSF shall return the VPLMN specific
		// mapped S-NSSAI values for the S-NSSAI values in `subscribedNssai`. But also `sNssaiForMapping` shall be
		// provided if `requestMapping` is set to true. In the implementation, the NSSF would return mapped S-NSSAIs
		// for S-NSSAIs in both `sNssaiForMapping` and `subscribedSnssai` if present

		if param.HomePlmnId == nil {
			problemDetail := "[Query Parameter] `home-plmn-id` should be provided when requesting VPLMN specific mapped S-NSSAI values"
			*problemDetails = models.ProblemDetails{
				Title:  util.INVALID_REQUEST,
				Status: http.StatusBadRequest,
				Detail: problemDetail,
				InvalidParams: []models.InvalidParam{
					{
						Param:  "home-plmn-id",
						Reason: problemDetail,
					},
				},
			}

			status = http.StatusBadRequest
			return status
		}

		mappingOfSnssai := util.GetMappingOfPlmnFromConfig(*param.HomePlmnId)

		if mappingOfSnssai != nil {
			// Find mappings for S-NSSAIs in `subscribedSnssai`
			for _, subscribedSnssai := range param.SliceInfoRequestForRegistration.SubscribedNssai {
				if util.CheckStandardSnssai(*subscribedSnssai.SubscribedSnssai) {
					continue
				}

				targetMapping, found := util.FindMappingWithHomeSnssai(*subscribedSnssai.SubscribedSnssai, mappingOfSnssai)

				if !found {
					logger.Nsselection.Warnf("No mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
						*subscribedSnssai.SubscribedSnssai,
						*param.HomePlmnId)
					continue
				} else {
					// Add mappings to Allowed NSSAI list
					var allowedSnssaiElement models.AllowedSnssai
					allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
					*allowedSnssaiElement.AllowedSnssai = *targetMapping.ServingSnssai
					allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
					*allowedSnssaiElement.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai

					// Default Access Type is set to 3GPP Access if no TAI is provided
					// TODO: Depend on operator implementation, it may also return S-NSSAIs in all valid Access Type if
					//       UE's Access Type could not be identified
					var accessType models.AccessType = models.AccessType__3_GPP_ACCESS
					if param.Tai != nil {
						accessType = util.GetAccessTypeFromConfig(*param.Tai)
					}

					util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)
				}
			}

			// Find mappings for S-NSSAIs in `sNssaiForMapping`
			for _, snssai := range param.SliceInfoRequestForRegistration.SNssaiForMapping {
				if util.CheckStandardSnssai(snssai) {
					continue
				}

				targetMapping, found := util.FindMappingWithHomeSnssai(snssai, mappingOfSnssai)

				if !found {
					logger.Nsselection.Warnf("No mapping of Subscribed S-NSSAI %+v in PLMN %+v in NSSF configuration",
						snssai,
						*param.HomePlmnId)
					continue
				} else {
					// Add mappings to Allowed NSSAI list
					var allowedSnssaiElement models.AllowedSnssai
					allowedSnssaiElement.AllowedSnssai = new(models.Snssai)
					*allowedSnssaiElement.AllowedSnssai = *targetMapping.ServingSnssai
					allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
					*allowedSnssaiElement.MappedHomeSnssai = snssai

					// Default Access Type is set to 3GPP Access if no TAI is provided
					// TODO: Depend on operator implementation, it may also return S-NSSAIs in all valid Access Type if
					//       UE's Access Type could not be identified
					var accessType models.AccessType = models.AccessType__3_GPP_ACCESS
					if param.Tai != nil {
						accessType = util.GetAccessTypeFromConfig(*param.Tai)
					}

					util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)
				}
			}

			status = http.StatusOK
			return status
		} else {
			logger.Nsselection.Warnf("No S-NSSAI mapping of UE's HPLMN %+v in NSSF configuration", *param.HomePlmnId)

			status = http.StatusOK
			return status
		}
	}

	if param.SliceInfoRequestForRegistration.RequestedNssai != nil &&
		len(param.SliceInfoRequestForRegistration.RequestedNssai) != 0 {
		// Requested NSSAI is provided
		// Verify which S-NSSAI(s) in the Requested NSSAI are permitted based on comparing the Subscribed S-NSSAI(s)

		// Check if any Requested S-NSSAIs is present in Subscribed S-NSSAIs
		checkIfRequestAllowed := false

		for _, requestedSnssai := range param.SliceInfoRequestForRegistration.RequestedNssai {
			if param.Tai != nil && !util.CheckSupportedSnssaiInTa(requestedSnssai, *param.Tai) {
				// Requested S-NSSAI does not supported in UE's current TA
				// Add it to Rejected NSSAI in TA
				authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, requestedSnssai)
				continue
			}

			var mappingOfRequestedSnssai models.Snssai
			// TODO: Compared with Restricted S-NSSAI list in configuration under roaming scenario
			if param.HomePlmnId != nil && !util.CheckStandardSnssai(requestedSnssai) {
				// Standard S-NSSAIs are supported to be commonly decided by all roaming partners
				// Only non-standard S-NSSAIs are required to find mappings
				targetMapping, found := util.FindMappingWithServingSnssai(requestedSnssai,
					param.SliceInfoRequestForRegistration.MappingOfNssai)

				if !found {
					// No mapping of Requested S-NSSAI to HPLMN S-NSSAI is provided by UE
					// TODO: Search for local configuration if there is no provided mapping from UE, and update UE's
					//       Configured NSSAI
					authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, requestedSnssai)
					continue
				} else {
					// TODO: Check if mappings of S-NSSAIs are correct
					//       If not, update UE's Configured NSSAI
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
					if param.HomePlmnId != nil && !util.CheckStandardSnssai(requestedSnssai) {
						allowedSnssaiElement.MappedHomeSnssai = new(models.Snssai)
						*allowedSnssaiElement.MappedHomeSnssai = *subscribedSnssai.SubscribedSnssai
					}

					// Default Access Type is set to 3GPP Access if no TAI is provided
					// TODO: Depend on operator implementation, it may also return S-NSSAIs in all valid Access Type if
					//       UE's Access Type could not be identified
					var accessType models.AccessType = models.AccessType__3_GPP_ACCESS
					if param.Tai != nil {
						accessType = util.GetAccessTypeFromConfig(*param.Tai)
					}

					util.AddAllowedSnssai(allowedSnssaiElement, accessType, authorizedNetworkSliceInfo)

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

	if param.Tai != nil &&
		!util.CheckAllowedNssaiInAmfTa(authorizedNetworkSliceInfo.AllowedNssaiList, param.NfId, *param.Tai) {
		util.AddAmfInformation(*param.Tai, authorizedNetworkSliceInfo)
	}

	if param.SliceInfoRequestForRegistration.DefaultConfiguredSnssaiInd {
		// Default Configured NSSAI Indication is received from AMF
		// Determine the Configured NSSAI based on the Default Configured NSSAI
		useDefaultConfiguredNssai(param, authorizedNetworkSliceInfo)
	}

	status = http.StatusOK
	return status
}
