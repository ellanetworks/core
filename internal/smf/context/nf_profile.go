package context

import (
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

var NFServices *[]models.NfService

var NfServiceVersion *[]models.NfServiceVersion

var SmfInfo *models.SmfInfo

type SmfSnssaiPlmnIdInfo map[string]models.PlmnId

var SmfPlmnInfo SmfSnssaiPlmnIdInfo

func SmfPlmnConfig() *[]models.PlmnId {
	plmns := make([]models.PlmnId, 0)
	for _, plmn := range SmfPlmnInfo {
		plmns = append(plmns, plmn)
	}

	if len(plmns) > 0 {
		logger.SmfLog.Debugf("plmnId configured [%v] ", plmns)
		return &plmns
	}
	return nil
}

func SNssaiSmfInfo() *[]models.SnssaiSmfInfoItem {
	snssaiInfo := make([]models.SnssaiSmfInfoItem, 0)
	SmfPlmnInfo = make(SmfSnssaiPlmnIdInfo)
	smfSnssaiInfo := GetSnssaiInfo()
	for _, snssai := range smfSnssaiInfo {
		var snssaiInfoModel models.SnssaiSmfInfoItem
		snssaiInfoModel.SNssai = &models.Snssai{
			Sst: snssai.Snssai.Sst,
			Sd:  snssai.Snssai.Sd,
		}

		// Plmn Info
		if snssai.PlmnId.Mcc != "" && snssai.PlmnId.Mnc != "" {
			SmfPlmnInfo[strconv.Itoa(int(snssai.Snssai.Sst))+snssai.Snssai.Sd] = snssai.PlmnId
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
