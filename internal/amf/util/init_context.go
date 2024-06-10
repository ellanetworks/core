package util

import (
	"os"

	"github.com/google/uuid"
	"github.com/omec-project/nas/security"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/drsm"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/logger"
)

func InitDrsm() (drsm.DrsmInterface, error) {
	podname := os.Getenv("HOSTNAME")
	podip := os.Getenv("POD_IP")
	logger.UtilLog.Infof("NfId Instance: %v", context.AMF_Self().NfId)
	podId := drsm.PodId{PodName: podname, PodInstance: context.AMF_Self().NfId, PodIp: podip}
	logger.UtilLog.Debugf("PodId: %v", podId)
	dbUrl := factory.AmfConfig.Configuration.Mongodb.Url
	opt := &drsm.Options{ResIdSize: 24, Mode: drsm.ResourceClient}
	db := drsm.DbInfo{Url: dbUrl, Name: factory.AmfConfig.Configuration.AmfDBName}

	// amfid is being used for amfngapid, subscriberid and tmsi for this release
	return drsm.InitDRSM("amfid", podId, db, opt)
}

func InitAmfContext(context *context.AMFContext) {
	config := factory.AmfConfig
	logger.UtilLog.Infof("amfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	context.NfId = uuid.New().String()
	context.Name = configuration.AmfName
	context.NgapIpList = configuration.NgapIpList
	context.NgapPort = configuration.NgapPort
	context.SctpGrpcPort = configuration.SctpGrpcPort
	sbi := configuration.Sbi
	context.UriScheme = models.UriScheme(sbi.Scheme)
	context.RegisterIPv4 = sbi.RegisterIPv4
	context.SBIPort = sbi.Port
	context.BindingIPv4 = sbi.BindingIPv4
	serviceNameList := configuration.ServiceNameList
	context.InitNFService(serviceNameList, config.Info.Version)
	context.ServedGuamiList = configuration.ServedGumaiList
	context.SupportTaiLists = configuration.SupportTAIList
	context.PlmnSupportList = configuration.PlmnSupportList
	context.SupportDnnLists = configuration.SupportDnnList
	context.NrfUri = configuration.NrfUri
	security := configuration.Security
	context.SecurityAlgorithm.IntegrityOrder = getIntAlgOrder(security.IntegrityOrder)
	context.SecurityAlgorithm.CipheringOrder = getEncAlgOrder(security.CipheringOrder)
	context.NetworkName = configuration.NetworkName
	context.T3502Value = configuration.T3502Value
	context.T3512Value = configuration.T3512Value
	context.Non3gppDeregistrationTimerValue = configuration.Non3gppDeregistrationTimerValue
	context.T3513Cfg = configuration.T3513
	context.T3522Cfg = configuration.T3522
	context.T3550Cfg = configuration.T3550
	context.T3560Cfg = configuration.T3560
	context.T3565Cfg = configuration.T3565
	context.EnableNrfCaching = configuration.EnableNrfCaching
}

func getIntAlgOrder(integrityOrder []string) (intOrder []uint8) {
	for _, intAlg := range integrityOrder {
		switch intAlg {
		case "NIA0":
			intOrder = append(intOrder, security.AlgIntegrity128NIA0)
		case "NIA1":
			intOrder = append(intOrder, security.AlgIntegrity128NIA1)
		case "NIA2":
			intOrder = append(intOrder, security.AlgIntegrity128NIA2)
		case "NIA3":
			intOrder = append(intOrder, security.AlgIntegrity128NIA3)
		default:
			logger.UtilLog.Errorf("Unsupported algorithm: %s", intAlg)
		}
	}
	return
}

func getEncAlgOrder(cipheringOrder []string) (encOrder []uint8) {
	for _, encAlg := range cipheringOrder {
		switch encAlg {
		case "NEA0":
			encOrder = append(encOrder, security.AlgCiphering128NEA0)
		case "NEA1":
			encOrder = append(encOrder, security.AlgCiphering128NEA1)
		case "NEA2":
			encOrder = append(encOrder, security.AlgCiphering128NEA2)
		case "NEA3":
			encOrder = append(encOrder, security.AlgCiphering128NEA3)
		default:
			logger.UtilLog.Errorf("Unsupported algorithm: %s", encAlg)
		}
	}
	return
}
