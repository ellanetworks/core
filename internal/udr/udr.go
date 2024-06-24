package udr

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/service"
)

var UDR = &service.UDR{}

const (
	COMMON_DB_NAME = "free5gc"
	AUTH_DB_NAME   = "authentication"
	SBI_PORT       = 29504
)

func Start(mongoDBURL string, nrfURL string, webuiURL string) error {
	configuration := factory.Configuration{
		Sbi: &factory.Sbi{
			RegisterIPv4: "0.0.0.0",
			BindingIPv4:  "0.0.0.0",
			Port:         SBI_PORT,
		},
		Mongodb: &factory.Mongodb{
			Name:           COMMON_DB_NAME,
			Url:            mongoDBURL,
			AuthKeysDbName: AUTH_DB_NAME,
			AuthUrl:        mongoDBURL,
		},
		NrfUri:   nrfURL,
		WebuiUri: webuiURL,
	}
	config := factory.Config{
		Info: &factory.Info{
			Description: "UDR initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			UDR: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		Configuration: &configuration,
	}
	UDR.Initialize(config)
	go UDR.Start()
	return nil
}
