package nrf

import (
	"fmt"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/nrf/factory"

	"github.com/yeastengine/ella/internal/nrf/service"
)

var NRF = &service.NRF{}

const (
	dBName = "nrf"
	port   = 29510
)

func Start(mongoDBURL string, webuiUrl string) (string, error) {
	configuration := factory.Configuration{
		Sbi: &factory.Sbi{
			BindingIPv4:  "0.0.0.0",
			Port:         port,
			RegisterIPv4: "0.0.0.0",
		},
		MongoDBName: dBName,
		MongoDBUrl:  mongoDBURL,
		WebuiUri:    webuiUrl,
		ServiceNameList: []string{
			"nnrf-nfm",
			"nnrf-disc",
		},
		NfKeepAliveTime:       60,
		NfProfileExpiryEnable: true,
	}
	config := factory.Config{
		Configuration: &configuration,
		Info: &factory.Info{
			Description: "NRF initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			NRF: &logger.LogSetting{
				ReportCaller: false,
				DebugLevel:   "debug",
			},
		},
	}
	err := NRF.Initialize(config)
	if err != nil {
		return "", fmt.Errorf("failed to initialize")
	}
	go NRF.Start()
	return "http://127.0.0.1:29510", nil
}
