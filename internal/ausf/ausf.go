package ausf

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/ausf/factory"
	"github.com/yeastengine/ella/internal/ausf/service"
)

const (
	SBI_PORT      = 29509
	AUSF_GROUP_ID = "ausfGroup001"
)

var AUSF = &service.AUSF{}

func Start(nrfUrl string, udnUrl string, webuiUrl string) error {
	configuration := factory.Configuration{
		Sbi: &factory.Sbi{
			RegisterIPv4: "0.0.0.0",
			BindingIPv4:  "0.0.0.0",
			Port:         SBI_PORT,
		},
		ServiceNameList: []string{
			"nausf-auth",
		},
		NrfUri:   nrfUrl,
		UdmUri:   udnUrl,
		WebuiUri: webuiUrl,
		GroupId:  AUSF_GROUP_ID,
	}

	config := factory.Config{
		Configuration: &configuration,
		Info: &factory.Info{
			Description: "AUSF initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			AUSF: &logger.LogSetting{
				ReportCaller: false,
				DebugLevel:   "debug",
			},
		},
	}
	AUSF.Initialize(config)
	go AUSF.Start()
	return nil
}
