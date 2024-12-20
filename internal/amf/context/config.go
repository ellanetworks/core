package context

import (
	"fmt"
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/factory"
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
	plmnID := models.PlmnId{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	sstString := dbNetwork.Sst
	sstInt64, err := strconv.ParseInt(sstString, 10, 32)
	if err != nil {
		logger.AmfLog.Warnf("Failed to parse sst: %s", err)
		return plmnSupportList
	}
	snssai := models.Snssai{
		Sst: int32(sstInt64),
		Sd:  dbNetwork.Sd,
	}
	plmnSupportItem := factory.PlmnSupportItem{
		PlmnId:     plmnID,
		SNssaiList: []models.Snssai{snssai},
	}
	plmnSupportList = append(plmnSupportList, plmnSupportItem)
	return plmnSupportList
}
