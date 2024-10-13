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
	logger.UtilLog.Infof("pcfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	context.NfId = uuid.New().String()
	if configuration.PcfName != "" {
		context.Name = configuration.PcfName
	}

	sbi := configuration.Sbi
	context.AmfUri = configuration.AmfUri
	context.NrfUri = configuration.NrfUri
	context.UdrUri = configuration.UdrUri
	context.SBIPort = sbi.Port
	context.UriScheme = models.UriScheme_HTTP
	context.BindingIPv4 = sbi.BindingIPv4
	serviceList := configuration.ServiceList
	context.PlmnList = configuration.PlmnList
	context.InitNFService(serviceList, config.Info.Version)
	context.TimeFormat = configuration.TimeFormat
	context.DefaultBdtRefId = configuration.DefaultBdtRefId
	for _, service := range context.NfService {
		var err error
		context.PcfServiceUris[service.ServiceName] = service.ApiPrefix + "/" + string(service.ServiceName) + "/" + (*service.Versions)[0].ApiVersionInUri
		context.PcfSuppFeats[service.ServiceName], err = openapi.NewSupportedFeature(service.SupportedFeatures)
		if err != nil {
			logger.UtilLog.Errorf("openapi NewSupportedFeature error: %+v", err)
		}
	}
}
