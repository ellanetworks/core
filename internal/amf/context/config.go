package context

import (
	"fmt"
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/webui/configapi"
)

// This file contains calls to webui to get configuration data

// GetSupportTaiList returns a list of supported TAI
func GetSupportTaiList() []models.Tai {
	tais := make([]models.Tai, 0)
	networkSliceNames := configapi.ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := configapi.GetNetworkSliceByName2(networkSliceName)
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
	networkSliceNames := configapi.ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := configapi.GetNetworkSliceByName2(networkSliceName)
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
	networkSliceNames := configapi.ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := configapi.GetNetworkSliceByName2(networkSliceName)
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
