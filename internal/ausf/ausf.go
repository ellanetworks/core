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

func Start(udmUrl string) error {
	configuration := factory.Configuration{
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		ServiceNameList: []string{
			"nausf-auth",
		},
		UdmUri:  udmUrl,
		GroupId: AUSF_GROUP_ID,
	}

	config := factory.Config{
		Configuration: &configuration,
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
