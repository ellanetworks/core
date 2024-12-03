package nms

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/nms/config"
	"github.com/yeastengine/ella/internal/nms/server"
)

var NMS = &server.NMS{}

const (
	GRPC_PORT  = "9876"
	ConfigPort = 5000
)

func Start() (string, error) {
	configuration := config.Configuration{
		Logger: &logger.Logger{
			WEBUI: &logger.LogSetting{
				DebugLevel: "debug",
			},
		},
		CfgPort: ConfigPort,
	}
	NMS.Initialize(configuration)
	go NMS.Start()
	return "localhost:" + GRPC_PORT, nil
}
