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
	networkSlices, err := amfSelf.DbInstance.ListNetworkSlices()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get network slice names: %s", err)
		return tais
	}
	for _, networkSlice := range networkSlices {
		plmnID := models.PlmnId{
			Mcc: networkSlice.Mcc,
			Mnc: networkSlice.Mnc,
		}
		gnbs, err := networkSlice.GetGNodeBs()
		if err != nil {
			logger.AmfLog.Warnf("Failed to get gNodeBs: %s", err)
			continue
		}
		tai := models.Tai{
			PlmnId: &plmnID,
			Tac:    fmt.Sprintf("%06x", gnbs[0].Tac),
		}
		tais = append(tais, tai)
	}
	return tais
}

func GetServedGuamiList() []models.Guami {
	amfSelf := AMF_Self()
	guamis := make([]models.Guami, 0)
	networkSlices, err := amfSelf.DbInstance.ListNetworkSlices()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get network slice names: %s", err)
		return guamis
	}
	for _, networkSlice := range networkSlices {
		plmnID := models.PlmnId{
			Mcc: networkSlice.Mcc,
			Mnc: networkSlice.Mnc,
		}
		guami := models.Guami{
			PlmnId: &plmnID,
			AmfId:  "cafe00", // To edit
		}
		guamis = append(guamis, guami)
	}
	return guamis
}

func GetPlmnSupportList() []factory.PlmnSupportItem {
	amfSelf := AMF_Self()
	plmnSupportList := make([]factory.PlmnSupportItem, 0)
	networkSlices, err := amfSelf.DbInstance.ListNetworkSlices()
	if err != nil {
		logger.AmfLog.Warnf("Failed to get network slice names: %s", err)
		return plmnSupportList
	}
	for _, networkSlice := range networkSlices {
		plmnID := models.PlmnId{
			Mcc: networkSlice.Mcc,
			Mnc: networkSlice.Mnc,
		}
		sstString := networkSlice.Sst
		sstInt64, err := strconv.ParseInt(sstString, 10, 32)
		if err != nil {
			continue
		}
		snssai := models.Snssai{
			Sst: int32(sstInt64),
			Sd:  networkSlice.Sd,
		}
		plmnSupportItem := factory.PlmnSupportItem{
			PlmnId:     plmnID,
			SNssaiList: []models.Snssai{snssai},
		}
		plmnSupportList = append(plmnSupportList, plmnSupportItem)
	}
	return plmnSupportList
}
