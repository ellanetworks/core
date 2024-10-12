package util

import (
	"time"

	"github.com/google/uuid"
	"github.com/omec-project/nas/security"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/logger"
)

func InitAmfContext(context *context.AMFContext) {
	config := factory.AmfConfig
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
	context.UriScheme = models.UriScheme_HTTP
	context.RegisterIPv4 = configuration.Sbi.RegisterIPv4
	context.SBIPort = sbi.Port
	context.BindingIPv4 = sbi.BindingIPv4
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
	context.AusfUri = configuration.AusfUri
	context.NrfUri = configuration.NrfUri
	context.NssfUri = configuration.NssfUri
	context.PcfUri = configuration.PcfUri
	context.SmfUri = configuration.SmfUri
	context.UdmsdmUri = configuration.UdmsdmUri
	context.UdmUecmUri = configuration.UdmUecmUri
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
