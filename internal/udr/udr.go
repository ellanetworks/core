package udr

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/service"
)

var UDR = &service.UDR{}

func Start() error {
	configuration := factory.Configuration{
		Logger: &logger.Logger{
			UDR: &logger.LogSetting{
				DebugLevel: "debug",
			},
		},
	}
	UDR.Initialize(configuration)
	return nil
}
