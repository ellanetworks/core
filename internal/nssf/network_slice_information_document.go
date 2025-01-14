// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nssf

import (
	"github.com/omec-project/openapi/models"
)

func GetNSSelection(param NsselectionQueryParameter) (*models.AuthorizedNetworkSliceInfo, error) {
	response := &models.AuthorizedNetworkSliceInfo{}
	if param.SliceInfoRequestForRegistration != nil {
		err := nsselectionForRegistration(param, response, nil)
		if err != nil {
			return nil, err
		}
	}

	if param.SliceInfoRequestForPduSession != nil {
		err := nsselectionForPduSession(param, response, nil)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}
