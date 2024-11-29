package util

import (
	"github.com/omec-project/openapi/models"
	udr_context "github.com/yeastengine/ella/internal/udr/context"
	"github.com/yeastengine/ella/internal/udr/factory"
)

func InitUdrContext(context *udr_context.UDRContext) {
	config := factory.UdrConfig
	sbi := config.Sbi
	context.UriScheme = models.UriScheme_HTTP
	context.SBIPort = sbi.Port
	context.BindingIPv4 = sbi.BindingIPv4
}
