package udr

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/service"
)

var UDR = &service.UDR{}

const SBI_PORT = 29504

func Start(mongoDBURL string, mongoDBName string) error {
	configuration := factory.Configuration{
		Logger: &logger.Logger{
			UDR: &logger.LogSetting{
				DebugLevel: "debug",
			},
		},
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		Mongodb: &factory.Mongodb{
			Name: mongoDBName,
			Url:  mongoDBURL,
		},
	}
	UDR.Initialize(configuration)
	go UDR.Start()
	return nil
}
