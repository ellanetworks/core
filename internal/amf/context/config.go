package context

import (
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

// This file contains calls to db to get configuration data

func GetSupportTaiList() []models.Tai {
	amfSelf := AMF_Self()
	tais := make([]models.Tai, 0)
	dbNetwork, err := amfSelf.DbInstance.GetNetwork()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get network slice names: %s", err)
		return tais
	}
	plmnID := models.PlmnId{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	radios, err := amfSelf.DbInstance.ListRadios()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get radios: %s", err)
		return tais
	}
	tai := models.Tai{
		PlmnId: &plmnID,
		Tac:    radios[0].Tac,
	}
	tais = append(tais, tai)
	return tais
}

func GetServedGuamiList() []models.Guami {
	amfSelf := AMF_Self()
	guamis := make([]models.Guami, 0)
	dbNetwork, err := amfSelf.DbInstance.GetNetwork()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get network slice names: %s", err)
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
	amfSelf := AMF_Self()
	plmnSupportList := make([]PlmnSupportItem, 0)
	dbNetwork, err := amfSelf.DbInstance.GetNetwork()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get network slice names: %s", err)
		return plmnSupportList
	}
	plmnSupportItem := PlmnSupportItem{
		PlmnId: models.PlmnId{
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
