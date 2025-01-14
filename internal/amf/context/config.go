// Copyright 2024 Ella Networks
package context

import (
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

// This file contains calls to db to get configuration data

func ListAmfRan() []AmfRan {
	amfSelf := AMFSelf()
	return amfSelf.ListAmfRan()
}

func GetSupportTaiList() []models.Tai {
	amfSelf := AMFSelf()
	tais := make([]models.Tai, 0)
	dbNetwork, err := amfSelf.DBInstance.GetOperator()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get operator: %s", err)
		return tais
	}
	plmnID := models.PlmnId{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	supportedTacs := dbNetwork.GetSupportedTacs()
	for _, tac := range supportedTacs {
		tai := models.Tai{
			PlmnId: &plmnID,
			Tac:    tac,
		}
		tais = append(tais, tai)
	}
	return tais
}

func GetServedGuamiList() []models.Guami {
	amfSelf := AMFSelf()
	guamis := make([]models.Guami, 0)
	dbNetwork, err := amfSelf.DBInstance.GetOperator()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get operator: %s", err)
		return guamis
	}
	plmnID := models.PlmnId{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	guami := models.Guami{
		PlmnId: &plmnID,
		AmfId:  "cafe00", // To edit
	}
	guamis = append(guamis, guami)
	return guamis
}

func GetPlmnSupportList() []PlmnSupportItem {
	amfSelf := AMFSelf()
	plmnSupportList := make([]PlmnSupportItem, 0)
	dbNetwork, err := amfSelf.DBInstance.GetOperator()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get operator: %s", err)
		return plmnSupportList
	}
	plmnSupportItem := PlmnSupportItem{
		PlmnID: models.PlmnId{
			Mcc: dbNetwork.Mcc,
			Mnc: dbNetwork.Mnc,
		},
		SNssaiList: []models.Snssai{
			{
				Sst: config.Sst,
				Sd:  config.Sd,
			},
		},
	}
	plmnSupportList = append(plmnSupportList, plmnSupportItem)
	return plmnSupportList
}
