package util

import (
	"os"

	"github.com/google/uuid"
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
	udmContext.NfId = uuid.New().String()
	if configuration.UdmName != "" {
		udmContext.Name = configuration.UdmName
	}
	sbi := configuration.Sbi
	udmContext.UriScheme = ""
	udmContext.SBIPort = factory.UDM_DEFAULT_PORT_INT
	udmContext.RegisterIPv4 = factory.UDM_DEFAULT_IPV4
	if sbi != nil {
		udmContext.UriScheme = models.UriScheme_HTTP
		if sbi.RegisterIPv4 != "" {
			udmContext.RegisterIPv4 = sbi.RegisterIPv4
		}
		if sbi.Port != 0 {
			udmContext.SBIPort = sbi.Port
		}

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
	}
	udmContext.NrfUri = configuration.NrfUri
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
