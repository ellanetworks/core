package nssf

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/service"
)

var NSSF = &service.NSSF{}

const (
	SD       = "102030"
	SST      = 1
	SBI_PORT = 29531
)

func Start() error {
	configuration := factory.Configuration{
		Logger: &logger.Logger{
			NSSF: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		NssfName: "NSSF",
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		ServiceNameList: []models.ServiceName{
			"nnssf-nsselection",
			"nnssf-nssaiavailability",
		},
	}
	NSSF.Initialize(configuration)
	go NSSF.Start()
	return nil
}
