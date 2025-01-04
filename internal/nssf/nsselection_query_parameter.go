// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nssf

import "github.com/omec-project/openapi/models"

type NsselectionQueryParameter struct {
	NfType                          *models.NfType                   `json:"nf-type"`
	NfId                            string                           `json:"nf-id"`
	SliceInfoRequestForRegistration *models.SliceInfoForRegistration `json:"slice-info-request-for-registration,omitempty"`
	SliceInfoRequestForPduSession   *models.SliceInfoForPduSession   `json:"slice-info-request-for-pdu-session,omitempty"`
	HomePlmnId                      *models.PlmnId                   `json:"home-plmn-id,omitempty"`
	Tai                             *models.Tai                      `json:"tai,omitempty"`
	SupportedFeatures               string                           `json:"supported-features,omitempty"`
}
