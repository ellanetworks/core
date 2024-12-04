package context

import (
	"fmt"
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/pcf/logger"
)

// This file contains calls to db to get configuration data

func GetSupportTaiList() []models.Tai {
	tais := make([]models.Tai, 0)
	networkSliceNames, err := queries.ListNetworkSliceNames()
	if err != nil {
		logger.CtxLog.Warnf("Failed to get network slice names: %s", err)
		return tais
	}
	for _, networkSliceName := range networkSliceNames {
		networkSlice, err := queries.GetNetworkSliceByName(networkSliceName)
		if err != nil {
			logger.CtxLog.Warnf("Failed to get network slice by name: %s", networkSliceName)
			continue
		}
		plmnID := models.PlmnId{
			Mcc: networkSlice.SiteInfo.Plmn.Mcc,
			Mnc: networkSlice.SiteInfo.Plmn.Mnc,
		}
		tai := models.Tai{
			PlmnId: &plmnID,
			Tac:    fmt.Sprintf("%06x", networkSlice.SiteInfo.GNodeBs[0].Tac),
		}
		tais = append(tais, tai)
	}
	return tais
}

func GetServedGuamiList() []models.Guami {
	guamis := make([]models.Guami, 0)
	networkSliceNames, err := queries.ListNetworkSliceNames()
	if err != nil {
		logger.CtxLog.Warnf("Failed to get network slice names: %s", err)
		return guamis
	}
	for _, networkSliceName := range networkSliceNames {
		networkSlice, err := queries.GetNetworkSliceByName(networkSliceName)
		if err != nil {
			logger.CtxLog.Warnf("Failed to get network slice by name: %s", networkSliceName)
			continue
		}
		plmnID := models.PlmnId{
			Mcc: networkSlice.SiteInfo.Plmn.Mcc,
			Mnc: networkSlice.SiteInfo.Plmn.Mnc,
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
	plmnSupportList := make([]factory.PlmnSupportItem, 0)
	networkSliceNames, err := queries.ListNetworkSliceNames()
	if err != nil {
		logger.CtxLog.Warnf("Failed to get network slice names: %s", err)
		return plmnSupportList
	}
	for _, networkSliceName := range networkSliceNames {
		networkSlice, err := queries.GetNetworkSliceByName(networkSliceName)
		if err != nil {
			logger.CtxLog.Warnf("Failed to get network slice by name: %s", networkSliceName)
			continue
		}
		plmnID := models.PlmnId{
			Mcc: networkSlice.SiteInfo.Plmn.Mcc,
			Mnc: networkSlice.SiteInfo.Plmn.Mnc,
		}
		sstString := networkSlice.SliceId.Sst
		sstInt64, err := strconv.ParseInt(sstString, 10, 32)
		if err != nil {
			continue
		}
		snssai := models.Snssai{
			Sst: int32(sstInt64),
			Sd:  networkSlice.SliceId.Sd,
		}
		plmnSupportItem := factory.PlmnSupportItem{
			PlmnId:     plmnID,
			SNssaiList: []models.Snssai{snssai},
		}
		plmnSupportList = append(plmnSupportList, plmnSupportItem)
	}
	return plmnSupportList
}
