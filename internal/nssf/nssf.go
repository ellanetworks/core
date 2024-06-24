package nssf

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/service"
)

var NSSF = &service.NSSF{}

const (
	SD       = "010203"
	SST      = 1
	SBI_PORT = 29531
)

func Start(nrfURL string, webuiURL string) error {
	configuration := factory.Configuration{
		NssfName: "NSSF",
		Sbi: &factory.Sbi{
			BindingIPv4:  "0.0.0.0",
			Port:         SBI_PORT,
			RegisterIPv4: "0.0.0.0",
		},
		ServiceNameList: []models.ServiceName{
			"nnssf-nsselection",
			"nnssf-nssaiavailability",
		},
		NrfUri:   nrfURL,
		WebuiUri: webuiURL,
		NsiList: []factory.NsiConfig{
			{
				NsiInformationList: []models.NsiInformation{
					{
						NrfId: nrfURL + "/nnrf-nfm/v1/nf-instances",
						NsiId: "22",
					},
				},
				Snssai: &models.Snssai{
					Sd:  SD,
					Sst: SST,
				},
			},
		},
	}
	config := factory.Config{
		Info: &factory.Info{
			Description: "NSSF initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			NSSF: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		Configuration: &configuration,
	}
	NSSF.Initialize(config)
	go NSSF.Start()
	return nil
}
