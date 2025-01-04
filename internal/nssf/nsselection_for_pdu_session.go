// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nssf

import (
	"fmt"
	"net/http"

	"github.com/omec-project/openapi/models"
)

// Network slice selection for PDU session
// The function is executed when the IE, `slice-info-for-pdu-session`, is provided in query parameters
func nsselectionForPduSession(param NsselectionQueryParameter, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo, problemDetails *models.ProblemDetails) error {
	if param.HomePlmnId != nil {
		// Check whether UE's Home PLMN is supported when UE is a roamer
		authorizedNetworkSliceInfo.RejectedNssaiInPlmn = append(authorizedNetworkSliceInfo.RejectedNssaiInPlmn, *param.SliceInfoRequestForPduSession.SNssai)
		return nil
	}

	if param.Tai != nil {
		// Check whether UE's current TA is supported when UE provides TAI
		authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, *param.SliceInfoRequestForPduSession.SNssai)
		return nil
	}

	if param.HomePlmnId != nil {
		if param.SliceInfoRequestForPduSession.RoamingIndication == models.RoamingIndication_NON_ROAMING {
			problemDetail := "`home-plmn-id` is provided, which contradicts `roamingIndication`:'NON_ROAMING'"
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

			return fmt.Errorf("NSSF No Response")
		}
	} else {
		if param.SliceInfoRequestForPduSession.RoamingIndication != models.RoamingIndication_NON_ROAMING {
			problemDetail := fmt.Sprintf("`home-plmn-id` is not provided, which contradicts `roamingIndication`:'%s'",
				string(param.SliceInfoRequestForPduSession.RoamingIndication))
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

			return fmt.Errorf("NSSF No Response")
		}
	}

	if param.Tai != nil {
		// Requested S-NSSAI does not supported in UE's current TA
		// Add it to Rejected NSSAI in TA
		authorizedNetworkSliceInfo.RejectedNssaiInTa = append(authorizedNetworkSliceInfo.RejectedNssaiInTa, *param.SliceInfoRequestForPduSession.SNssai)
		return nil
	}

	*authorizedNetworkSliceInfo = models.AuthorizedNetworkSliceInfo{}

	return nil
}
