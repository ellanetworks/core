package webui

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/webui/backend/factory"
	"github.com/yeastengine/ella/internal/webui/backend/webui_service"
)

var WEBUI = &webui_service.WEBUI{}

const (
	GRPC_PORT  = "9876"
	ConfigPort = 5000
)

func Start(dbUrl string, dbName string) (string, error) {
	configuration := factory.Configuration{
		Logger: &logger.Logger{
			WEBUI: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		Mongodb: &factory.Mongodb{
			Name:           dbName,
			Url:            dbUrl,
			AuthKeysDbName: dbName,
			AuthUrl:        dbUrl,
		},
		CfgPort: ConfigPort,
	}
	WEBUI.Initialize(configuration)
	go WEBUI.Start()
	return "localhost:" + GRPC_PORT, nil
}
