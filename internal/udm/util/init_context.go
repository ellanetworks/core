package util

import (
	"os"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/util_3gpp/suci"
	"github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/factory"
	"github.com/yeastengine/ella/internal/udm/logger"
)

func InitUDMContext(udmContext *context.UDMContext) {
	config := factory.UdmConfig
	logger.UtilLog.Info("udmconfig Info: Version[", config.Info.Version, "] Description[", config.Info.Description, "]")
	configuration := config.Configuration
	if configuration.UdmName != "" {
		udmContext.Name = configuration.UdmName
	}
	sbi := configuration.Sbi
	udmContext.UriScheme = ""
	udmContext.UriScheme = models.UriScheme_HTTP
	udmContext.SBIPort = sbi.Port

	udmContext.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
	if udmContext.BindingIPv4 != "" {
		logger.UtilLog.Info("Parsing ServerIPv4 address from ENV Variable.")
	} else {
		udmContext.BindingIPv4 = sbi.BindingIPv4
		if udmContext.BindingIPv4 == "" {
			logger.UtilLog.Warn("Error parsing ServerIPv4 address as string. Using the 0.0.0.0 address as default.")
			udmContext.BindingIPv4 = "0.0.0.0"
		}
	}
	udmContext.UdrUri = configuration.UdrUri
	servingNameList := configuration.ServiceNameList

	udmContext.SuciProfiles = []suci.SuciProfile{
		{
			ProtectionScheme: "1", // Standard defined value for Protection Scheme A (TS 33.501 Annex C)
			PrivateKey:       configuration.Keys.UdmProfileAHNPrivateKey,
			PublicKey:        configuration.Keys.UdmProfileAHNPublicKey,
		},
		{
			ProtectionScheme: "2", // Standard defined value for Protection Scheme B (TS 33.501 Annex C)
			PrivateKey:       configuration.Keys.UdmProfileBHNPrivateKey,
			PublicKey:        configuration.Keys.UdmProfileBHNPublicKey,
		},
	}
	udmContext.PlmnList = configuration.PlmnList
	udmContext.InitNFService(servingNameList, config.Info.Version)
}
