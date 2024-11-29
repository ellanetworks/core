package util

import (
	"github.com/google/uuid"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/logger"
)

// InitpcfContext Init PCF Context from config file
func InitpcfContext(context *context.PCFContext) {
	config := factory.PcfConfig
	context.NfId = uuid.New().String()
	context.Name = config.PcfName
	sbi := config.Sbi
	context.AmfUri = config.AmfUri
	context.UdrUri = config.UdrUri
	context.SBIPort = sbi.Port
	context.UriScheme = models.UriScheme_HTTP
	context.BindingIPv4 = sbi.BindingIPv4
	serviceList := config.ServiceList
	context.PlmnList = config.PlmnList
	context.InitNFService(serviceList)
	context.TimeFormat = config.TimeFormat
	context.DefaultBdtRefId = config.DefaultBdtRefId
	for _, service := range context.NfService {
		var err error
		context.PcfSuppFeats[service.ServiceName], err = openapi.NewSupportedFeature(service.SupportedFeatures)
		if err != nil {
			logger.UtilLog.Errorf("openapi NewSupportedFeature error: %+v", err)
		}
	}
}
