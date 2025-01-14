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

	*authorizedNetworkSliceInfo = models.AuthorizedNetworkSliceInfo{}

	return nil
}
