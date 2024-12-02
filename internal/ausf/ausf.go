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
		Logger: &logger.Logger{
			AUSF: &logger.LogSetting{
				DebugLevel: "debug",
			},
		},
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

	AUSF.Initialize(configuration)
	go AUSF.Start()
	return nil
}
