package pcf

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/service"
)

var PCF = &service.PCF{}

func Start(amfURL string) error {
	configuration := factory.Configuration{
		Logger: &logger.Logger{
			PCF: &logger.LogSetting{
				DebugLevel: "debug",
			},
		},
		PcfName:         "PCF",
		DefaultBdtRefId: "BdtPolicyId-",
		AmfUri:          amfURL,
	}
	PCF.Initialize(configuration)
	go PCF.Start()
	return nil
}
