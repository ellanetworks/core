// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"github.com/omec-project/openapi/models"
)

var serviceUriMap = map[models.ServiceName]string{
	models.ServiceName_NPCF_AM_POLICY_CONTROL:   "policies",
	models.ServiceName_NPCF_SMPOLICYCONTROL:     "sm-policies",
	models.ServiceName_NPCF_BDTPOLICYCONTROL:    "bdtpolicies",
	models.ServiceName_NPCF_POLICYAUTHORIZATION: "app-sessions",
}

// GetSMPolicyDnnData returns SMPolicyDnnData derived from SmPolicy data which snssai and dnn match
func GetSMPolicyDnnData(data models.SmPolicyData, snssai *models.Snssai, dnn string) (result *models.SmPolicyDnnData) {
	if snssai == nil || dnn == "" || data.SmPolicySnssaiData == nil {
		return
	}
	snssaiString := SnssaiModelsToHex(*snssai)
	if snssaiData, exist := data.SmPolicySnssaiData[snssaiString]; exist {
		if snssaiData.SmPolicyDnnData == nil {
			return
		}
		if dnnInfo, exist := snssaiData.SmPolicyDnnData[dnn]; exist {
			result = &dnnInfo
			return
		}
	}
	return
}
