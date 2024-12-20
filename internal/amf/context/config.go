package context

import (
	"fmt"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/logger"
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
	gnbs, err := dbNetwork.GetGNodeBs()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get gNodeBs: %s", err)
		return tais
	}
	tai := models.Tai{
		PlmnId: &plmnID,
		Tac:    fmt.Sprintf("%06x", gnbs[0].Tac),
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

func GetPlmnSupportList() []factory.PlmnSupportItem {
	amfSelf := AMF_Self()
	plmnSupportList := make([]factory.PlmnSupportItem, 0)
	dbNetwork, err := amfSelf.DbInstance.GetNetwork()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get network slice names: %s", err)
		return plmnSupportList
	}
	plmnSupportItem := factory.PlmnSupportItem{
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
