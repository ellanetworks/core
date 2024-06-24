package webui

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/webui/backend/factory"
	"github.com/yeastengine/ella/internal/webui/backend/webui_service"
)

var WEBUI = &webui_service.WEBUI{}

const (
	dBName     = "free5gc"
	authDbName = "authentication"
	GRPC_PORT  = "9876"
	ConfigPort = 5000
)

func Start(dbUrl string) (string, error) {
	configuration := factory.Configuration{
		Mongodb: &factory.Mongodb{
			Name:           dBName,
			Url:            dbUrl,
			AuthKeysDbName: authDbName,
			AuthUrl:        dbUrl,
		},
		CfgPort: ConfigPort,
	}
	config := factory.Config{
		Info: &factory.Info{
			Description: "WebUI initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			WEBUI: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		Configuration: &configuration,
	}
	WEBUI.Initialize(config)
	go WEBUI.Start()
	return "localhost:" + GRPC_PORT, nil
}
