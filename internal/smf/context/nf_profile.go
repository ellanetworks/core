package context

import (
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/smf/logger"
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
		logger.CfgLog.Debugf("plmnId configured [%v] ", plmns)
		return &plmns
	}
	return nil
}

func SNssaiSmfInfo() *[]models.SnssaiSmfInfoItem {
	snssaiInfo := make([]models.SnssaiSmfInfoItem, 0)
	SmfPlmnInfo = make(SmfSnssaiPlmnIdInfo)
	for _, snssai := range smfContext.SnssaiInfos {
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
