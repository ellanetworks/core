// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nssf

import "github.com/omec-project/openapi/models"

type NsselectionQueryParameter struct {
	SliceInfoRequestForRegistration *models.SliceInfoForRegistration
	SliceInfoRequestForPduSession   *models.SliceInfoForPduSession
}
