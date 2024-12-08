package util

import (
	"github.com/google/uuid"
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/factory"
)

// InitpcfContext Init PCF Context from config file
func InitpcfContext(context *context.PCFContext) {
	config := factory.PcfConfig
	context.NfId = uuid.New().String()
	context.Name = config.PcfName
	context.AmfUri = config.AmfUri
	context.TimeFormat = config.TimeFormat
	context.DefaultBdtRefId = config.DefaultBdtRefId
}
