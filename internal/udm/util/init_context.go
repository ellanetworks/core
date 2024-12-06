package util

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/util_3gpp/suci"
	"github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/factory"
)

func InitUDMContext(udmContext *context.UDMContext) {
	config := factory.UdmConfig
	udmContext.Name = config.UdmName
	sbi := config.Sbi
	udmContext.UriScheme = ""
	udmContext.UriScheme = models.UriScheme_HTTP
	udmContext.SBIPort = sbi.Port
	udmContext.BindingIPv4 = sbi.BindingIPv4
	servingNameList := config.ServiceNameList

	udmContext.SuciProfiles = []suci.SuciProfile{
		{
			ProtectionScheme: "1", // Standard defined value for Protection Scheme A (TS 33.501 Annex C)
			PrivateKey:       config.Keys.UdmProfileAHNPrivateKey,
			PublicKey:        config.Keys.UdmProfileAHNPublicKey,
		},
		{
			ProtectionScheme: "2", // Standard defined value for Protection Scheme B (TS 33.501 Annex C)
			PrivateKey:       config.Keys.UdmProfileBHNPrivateKey,
			PublicKey:        config.Keys.UdmProfileBHNPublicKey,
		},
	}
	udmContext.PlmnList = config.PlmnList
	udmContext.InitNFService(servingNameList)
}
