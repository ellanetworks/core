package smf

import (
	"fmt"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/service"
)

var SMF = &service.SMF{}

const (
	dBName   = "smf"
	SBI_PORT = 29502
)

func Start(mongoDBURL string, nrfURL string, webuiURL string) error {
	configuration := factory.Configuration{
		Mongodb: &factory.Mongodb{
			Name: dBName,
			Url:  mongoDBURL,
		},
		PFCP: &factory.PFCP{
			Addr: "0.0.0.0",
		},
		Sbi: &factory.Sbi{
			RegisterIPv4: "0.0.0.0",
			BindingIPv4:  "0.0.0.0",
			Port:         SBI_PORT,
		},
		NrfUri:    nrfURL,
		WebuiUri:  webuiURL,
		SmfName:   "SMF",
		SmfDbName: dBName,
		ServiceNameList: []string{
			"nsmf-pdusession",
			"nsmf-event-exposure",
			"nsmf-oam",
		},
	}

	config := factory.Config{
		Configuration: &configuration,
		Info: &factory.Info{
			Description: "SMF initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			SMF: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
	}

	ueRoutingConfig := factory.RoutingConfig{
		Info: &factory.Info{
			Description: "Routing information for UE",
			Version:     "1.0.0",
		},
	}

	err := SMF.Initialize(config, ueRoutingConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize SMF")
	}
	go SMF.Start()
	return nil
}
