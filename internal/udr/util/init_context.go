package util

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/models"
	udr_context "github.com/yeastengine/ella/internal/udr/context"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/logger"
)

func InitUdrContext(context *udr_context.UDRContext) {
	config := factory.UdrConfig
	logger.UtilLog.Infof("udrconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	context.NfId = uuid.New().String()
	sbi := configuration.Sbi
	context.UriScheme = models.UriScheme_HTTP
	context.RegisterIPv4 = sbi.RegisterIPv4
	context.SBIPort = sbi.Port

	context.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
	if context.BindingIPv4 != "" {
		logger.UtilLog.Info("Parsing ServerIPv4 address from ENV Variable.")
	} else {
		context.BindingIPv4 = sbi.BindingIPv4
		if context.BindingIPv4 == "" {
			logger.UtilLog.Warn("Error parsing ServerIPv4 address as string. Using the 0.0.0.0 address as default.")
			context.BindingIPv4 = "0.0.0.0"
		}
	}
	if configuration.NrfUri != "" {
		context.NrfUri = configuration.NrfUri
	} else {
		logger.UtilLog.Warn("NRF Uri is empty! Using localhost as NRF IPv4 address.")
		context.NrfUri = fmt.Sprintf("%s://%s:%d", context.UriScheme, "127.0.0.1", 29510)
	}
}
