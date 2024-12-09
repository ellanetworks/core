package util

import (
	"github.com/omec-project/nas/security"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/logger"
)

func InitAmfContext(context *context.AMFContext) {
	config := factory.AmfConfig
	context.Name = config.AmfName
	context.NgapIpList = config.NgapIpList
	context.NgapPort = config.NgapPort
	context.SctpGrpcPort = config.SctpGrpcPort
	context.UriScheme = models.UriScheme_HTTP
	context.SupportDnnLists = config.SupportDnnList
	security := config.Security
	context.SecurityAlgorithm.IntegrityOrder = getIntAlgOrder(security.IntegrityOrder)
	context.SecurityAlgorithm.CipheringOrder = getEncAlgOrder(security.CipheringOrder)
	context.NetworkName = config.NetworkName
	context.T3502Value = config.T3502Value
	context.T3512Value = config.T3512Value
	context.Non3gppDeregistrationTimerValue = config.Non3gppDeregistrationTimerValue
	context.T3513Cfg = config.T3513
	context.T3522Cfg = config.T3522
	context.T3550Cfg = config.T3550
	context.T3560Cfg = config.T3560
	context.T3565Cfg = config.T3565
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
			logger.AmfLog.Errorf("Unsupported algorithm: %s", intAlg)
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
			logger.AmfLog.Errorf("Unsupported algorithm: %s", encAlg)
		}
	}
	return
}
