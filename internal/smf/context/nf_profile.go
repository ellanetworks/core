package context

import (
	"github.com/omec-project/openapi/models"
)

var NFServices *[]models.NfService

var NfServiceVersion *[]models.NfServiceVersion

var SmfInfo *models.SmfInfo

type SmfSnssaiPlmnIdInfo map[string]models.PlmnId

var SmfPlmnInfo SmfSnssaiPlmnIdInfo

// func SetupNFProfile(config *factory.Config) {
// 	// Set time
// 	nfSetupTime := time.Now()

// 	// set NfServiceVersion
// 	NfServiceVersion = &[]models.NfServiceVersion{
// 		{
// 			ApiVersionInUri: "v1",
// 			ApiFullVersion:  fmt.Sprintf("https://%s:%d/nsmf-pdusession/v1", SMF_Self().RegisterIPv4, SMF_Self().SBIPort),
// 			Expiry:          &nfSetupTime,
// 		},
// 	}

// 	// set smfInfo/PlmnInfo
// 	SmfInfo = &models.SmfInfo{
// 		SNssaiSmfInfoList: SNssaiSmfInfo(),
// 	}

// 	// set NFServices
// 	NFServices = new([]models.NfService)
// 	for _, serviceName := range config.Configuration.ServiceNameList {
// 		*NFServices = append(*NFServices, models.NfService{
// 			ServiceInstanceId: SMF_Self().NfInstanceID + serviceName,
// 			ServiceName:       models.ServiceName(serviceName),
// 			Versions:          NfServiceVersion,
// 			Scheme:            models.UriScheme_HTTP,
// 			NfServiceStatus:   models.NfServiceStatus_REGISTERED,
// 			ApiPrefix:         fmt.Sprintf("%s://%s:%d", SMF_Self().URIScheme, SMF_Self().RegisterIPv4, SMF_Self().SBIPort),
// 			AllowedPlmns:      SmfPlmnConfig(),
// 		})
// 	}
// }

// func SmfPlmnConfig() *[]models.PlmnId {
// 	plmns := make([]models.PlmnId, 0)
// 	for _, plmn := range SmfPlmnInfo {
// 		plmns = append(plmns, plmn)
// 	}

// 	if len(plmns) > 0 {
// 		logger.CfgLog.Debugf("plmnId configured [%v] ", plmns)
// 		return &plmns
// 	}
// 	return nil
// }

// func SNssaiSmfInfo() *[]models.SnssaiSmfInfoItem {
// 	snssaiInfo := make([]models.SnssaiSmfInfoItem, 0)
// 	SmfPlmnInfo = make(SmfSnssaiPlmnIdInfo)
// 	snssaiInfos, err := GetSnssaiInfos()
// 	if err != nil {
// 		logger.CfgLog.Errorf("GetSnssaiInfos failed: %v", err)
// 		return nil
// 	}
// 	for _, snssai := range snssaiInfos {
// 		var snssaiInfoModel models.SnssaiSmfInfoItem
// 		snssaiInfoModel.SNssai = &models.Snssai{
// 			Sst: snssai.Snssai.Sst,
// 			Sd:  snssai.Snssai.Sd,
// 		}

// 		// Plmn Info
// 		if snssai.PlmnId.Mcc != "" && snssai.PlmnId.Mnc != "" {
// 			SmfPlmnInfo[strconv.Itoa(int(snssai.Snssai.Sst))+snssai.Snssai.Sd] = snssai.PlmnId
// 		}

// 		dnnModelList := make([]models.DnnSmfInfoItem, 0)
// 		for dnn := range snssai.DnnInfos {
// 			dnnModelList = append(dnnModelList, models.DnnSmfInfoItem{
// 				Dnn: dnn,
// 			})
// 		}

// 		snssaiInfoModel.DnnSmfInfoList = &dnnModelList

// 		snssaiInfo = append(snssaiInfo, snssaiInfoModel)
// 	}

// 	return &snssaiInfo
// }
