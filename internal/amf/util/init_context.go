package util

import (
	"os"
	"time"

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
	dbUrl := "mongodb://mongodb-arbiter-headless"
	if factory.AmfConfig.Configuration.Mongodb != nil &&
		factory.AmfConfig.Configuration.Mongodb.Url != "" {
		dbUrl = factory.AmfConfig.Configuration.Mongodb.Url
	}
	opt := &drsm.Options{ResIdSize: 24, Mode: drsm.ResourceClient}
	db := drsm.DbInfo{Url: dbUrl, Name: factory.AmfConfig.Configuration.AmfDBName}

	// amfid is being used for amfngapid, subscriberid and tmsi for this release
	return drsm.InitDRSM("amfid", podId, db, opt)
}

func InitAmfContext(context *context.AMFContext) {
	config := factory.AmfConfig
	logger.UtilLog.Infof("amfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	if context.NfId == "" {
		context.NfId = uuid.New().String()
	}

	if configuration.AmfName != "" {
		context.Name = configuration.AmfName
	}
	if configuration.NgapIpList != nil {
		context.NgapIpList = configuration.NgapIpList
	} else {
		context.NgapIpList = []string{"127.0.0.1"} // default localhost
	}
	context.NgapPort = configuration.NgapPort
	context.SctpGrpcPort = configuration.SctpGrpcPort
	sbi := configuration.Sbi
	if sbi.Scheme != "" {
		context.UriScheme = models.UriScheme(sbi.Scheme)
	} else {
		logger.UtilLog.Warnln("SBI Scheme has not been set. Using http as default")
		context.UriScheme = "http"
	}
	context.RegisterIPv4 = factory.AMF_DEFAULT_IPV4 // default localhost
	context.SBIPort = factory.AMF_DEFAULT_PORT_INT  // default port
	if sbi != nil {
		if sbi.RegisterIPv4 != "" {
			context.RegisterIPv4 = os.Getenv("POD_IP")
		}
		if sbi.Port != 0 {
			context.SBIPort = sbi.Port
		}
		context.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
		if context.BindingIPv4 != "" {
			logger.UtilLog.Info("Parsing ServerIPv4 address from ENV Variable.")
		} else {
			context.BindingIPv4 = sbi.BindingIPv4
			if context.BindingIPv4 == "" {
				logger.UtilLog.Warn("Error parsing ServerIPv4 address from string. Using the 0.0.0.0 as default.")
				context.BindingIPv4 = "0.0.0.0"
			}
		}
	}
	serviceNameList := configuration.ServiceNameList
	context.InitNFService(serviceNameList, config.Info.Version)
	context.ServedGuamiList = configuration.ServedGumaiList
	context.SupportTaiLists = configuration.SupportTAIList
	// Tac value not converting into 3bytes hex string.
	// keeping tac integer value in string format received from configuration
	/*for i := range context.SupportTaiLists {
		if str := TACConfigToModels(context.SupportTaiLists[i].Tac); str != "" {
			context.SupportTaiLists[i].Tac = str
		}
	}*/
	context.PlmnSupportList = configuration.PlmnSupportList
	context.SupportDnnLists = configuration.SupportDnnList
	if configuration.NrfUri != "" {
		context.NrfUri = configuration.NrfUri
	} else {
		logger.UtilLog.Warn("NRF Uri is empty! Using localhost as NRF IPv4 address.")
		context.NrfUri = factory.AMF_DEFAULT_NRFURI
	}
	security := configuration.Security
	if security != nil {
		context.SecurityAlgorithm.IntegrityOrder = getIntAlgOrder(security.IntegrityOrder)
		context.SecurityAlgorithm.CipheringOrder = getEncAlgOrder(security.CipheringOrder)
	}
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
	if configuration.EnableNrfCaching {
		if configuration.NrfCacheEvictionInterval == 0 {
			context.NrfCacheEvictionInterval = time.Duration(900) // 15 mins
		} else {
			context.NrfCacheEvictionInterval = time.Duration(configuration.NrfCacheEvictionInterval)
		}
	}
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
