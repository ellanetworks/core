package db

import (
	"context"
	"fmt"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/factory"
)

func GetPLMNSupportList() ([]factory.PlmnSupportItem, error) {
	plmnSupport := []factory.PlmnSupportItem{}
	queries := factory.AmfConfig.Configuration.DBQueries
	networkSlices, err := queries.ListNetworkSlices(context.Background())
	if err != nil {
		return nil, fmt.Errorf("couldn't list network slices: %+v", err)
	}
	for _, networkSlice := range networkSlices {
		plmnId := models.PlmnId{
			Mcc: networkSlice.Mcc,
			Mnc: networkSlice.Mnc,
		}

		sstInt32 := int32(networkSlice.Sst)

		snssai := models.Snssai{
			Sst: sstInt32,
			Sd:  networkSlice.Sd,
		}
		pLMNSupportItem := factory.PlmnSupportItem{
			PlmnId:     plmnId,
			SNssaiList: []models.Snssai{snssai},
		}
		plmnSupport = append(plmnSupport, pLMNSupportItem)
	}
	return plmnSupport, nil
}

func GetSupportTaiList() ([]models.Tai, error) {
	taiList := []models.Tai{}
	queries := factory.AmfConfig.Configuration.DBQueries
	radiosList, err := queries.ListRadios(context.Background())
	if err != nil {
		return nil, fmt.Errorf("couldn't list radios: %+v", err)
	}
	for _, radio := range radiosList {
		networkSliceID := radio.NetworkSliceID
		networkSlice, err := queries.GetNetworkSlice(context.Background(), networkSliceID.Int64)
		if err != nil {
			return nil, fmt.Errorf("couldn't get network slice: %+v", err)
		}

		tai := models.Tai{
			PlmnId: &models.PlmnId{
				Mcc: networkSlice.Mcc,
				Mnc: networkSlice.Mnc,
			},
			Tac: radio.Tac,
		}
		taiList = append(taiList, tai)
	}

	return taiList, nil
}
