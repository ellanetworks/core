// Copyright 2024 Ella Networks
package context

import (
	"github.com/ellanetworks/core/internal/logger"
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/openapi/models"
)

// This file contains calls to db to get configuration data

func ListAmfRan() []AmfRan {
	amfSelf := AMF_Self()
	return amfSelf.ListAmfRan()
}

func GetSupportTaiList() []coreModels.Tai {
	amfSelf := AMF_Self()
	tais := make([]coreModels.Tai, 0)
	dbNetwork, err := amfSelf.DbInstance.GetOperator()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get operator: %s", err)
		return tais
	}
	plmnID := coreModels.PlmnId{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	supportedTacs := dbNetwork.GetSupportedTacs()
	for _, tac := range supportedTacs {
		tai := coreModels.Tai{
			PlmnId: &plmnID,
			Tac:    tac,
		}
		tais = append(tais, tai)
	}
	return tais
}

func GetServedGuamiList() []coreModels.Guami {
	amfSelf := AMF_Self()
	guamis := make([]coreModels.Guami, 0)
	dbNetwork, err := amfSelf.DbInstance.GetOperator()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get operator: %s", err)
		return guamis
	}
	plmnID := coreModels.PlmnId{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	guami := coreModels.Guami{
		PlmnId: &plmnID,
		AmfId:  "cafe00", // To edit
	}
	guamis = append(guamis, guami)
	return guamis
}

func GetPlmnSupportList() []PlmnSupportItem {
	amfSelf := AMF_Self()
	plmnSupportList := make([]PlmnSupportItem, 0)
	operator, err := amfSelf.DbInstance.GetOperator()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get operator: %s", err)
		return plmnSupportList
	}
	plmnSupportItem := PlmnSupportItem{
		PlmnId: models.PlmnId{
			Mcc: operator.Mcc,
			Mnc: operator.Mnc,
		},
		SNssaiList: []models.Snssai{
			{
				Sst: operator.Sst,
				Sd:  operator.GetHexSd(),
			},
		},
	}
	plmnSupportList = append(plmnSupportList, plmnSupportItem)
	return plmnSupportList
}
