// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func SNssaiToModels(ngapSnssai ngapType.SNSSAI) models.Snssai {
	var modelsSnssai models.Snssai

	modelsSnssai.Sst = int32(ngapSnssai.SST.Value[0])
	if ngapSnssai.SD != nil {
		modelsSnssai.Sd = hex.EncodeToString(ngapSnssai.SD.Value)
	}

	return modelsSnssai
}
