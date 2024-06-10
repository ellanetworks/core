package util

import (
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
	context.RegisterIPv4 = factory.UDR_DEFAULT_IPV4 // default localhost
	context.SBIPort = factory.UDR_DEFAULT_PORT_INT  // default port
	sbi := configuration.Sbi
	context.UriScheme = models.UriScheme(sbi.Scheme)
	context.RegisterIPv4 = sbi.RegisterIPv4
	context.SBIPort = sbi.Port
	context.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
	context.BindingIPv4 = sbi.BindingIPv4
	context.NrfUri = configuration.NrfUri
}
