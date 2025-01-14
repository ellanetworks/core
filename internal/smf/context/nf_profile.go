// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

var NFServices *[]models.NfService

var NfServiceVersion *[]models.NfServiceVersion

var SmfInfo *models.SmfInfo

type SmfSnssaiPlmnIDInfo map[string]models.PlmnId

var SmfPlmnInfo SmfSnssaiPlmnIDInfo

func SmfPlmnConfig() *[]models.PlmnId {
	plmns := make([]models.PlmnId, 0)
	for _, plmn := range SmfPlmnInfo {
		plmns = append(plmns, plmn)
	}

	if len(plmns) > 0 {
		logger.SmfLog.Debugf("plmnID configured [%v] ", plmns)
		return &plmns
	}
	return nil
}

func SNssaiSmfInfo() *[]models.SnssaiSmfInfoItem {
	snssaiInfo := make([]models.SnssaiSmfInfoItem, 0)
	SmfPlmnInfo = make(SmfSnssaiPlmnIDInfo)
	smfSnssaiInfo := GetSnssaiInfo()
	for _, snssai := range smfSnssaiInfo {
		var snssaiInfoModel models.SnssaiSmfInfoItem
		snssaiInfoModel.SNssai = &models.Snssai{
			Sst: snssai.Snssai.Sst,
			Sd:  snssai.Snssai.Sd,
		}

		// Plmn Info
		if snssai.PlmnID.Mcc != "" && snssai.PlmnID.Mnc != "" {
			SmfPlmnInfo[strconv.Itoa(int(snssai.Snssai.Sst))+snssai.Snssai.Sd] = snssai.PlmnID
		}

		dnnModelList := make([]models.DnnSmfInfoItem, 0)
		for dnn := range snssai.DnnInfos {
			dnnModelList = append(dnnModelList, models.DnnSmfInfoItem{
				Dnn: dnn,
			})
		}

		snssaiInfoModel.DnnSmfInfoList = &dnnModelList

		snssaiInfo = append(snssaiInfo, snssaiInfoModel)
	}

	return &snssaiInfo
}
