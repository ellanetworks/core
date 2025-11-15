// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
)

// GetSMPolicyDnnData returns SMPolicyDnnData derived from SmPolicy data which snssai and dnn match
func GetSMPolicyDnnData(data models.SmPolicyData, snssai *models.Snssai, dnn string) (*models.SmPolicyDnnData, error) {
	if snssai == nil {
		return nil, fmt.Errorf("snssai is nil")
	}
	if dnn == "" {
		return nil, fmt.Errorf("dnn is empty")
	}
	if data.SmPolicySnssaiData == nil {
		return nil, fmt.Errorf("sm policy data is nil")
	}

	snssaiString := fmt.Sprintf("%d%s", snssai.Sst, snssai.Sd)

	if snssaiData, exist := data.SmPolicySnssaiData[snssaiString]; exist {
		if snssaiData.SmPolicyDnnData == nil {
			return nil, fmt.Errorf("sm policy dnn data is nil")
		}
		if dnnInfo, exist := snssaiData.SmPolicyDnnData[dnn]; exist {
			return &dnnInfo, nil
		}
	}
	return nil, fmt.Errorf("no matching SmPolicyDnnData for snssai %s and dnn %s", snssaiString, dnn)
}
