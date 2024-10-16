package nssf

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/db/sql"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/service"
)

var NSSF = &service.NSSF{}

const (
	SD       = "102030"
	SST      = 1
	SBI_PORT = 29531
)

func Start(dbQueries *sql.Queries) error {
	configuration := factory.Configuration{
		NssfName:  "NSSF",
		DBQueries: dbQueries,
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		ServiceNameList: []models.ServiceName{
			"nnssf-nsselection",
			"nnssf-nssaiavailability",
		},
		NsiList: []factory.NsiConfig{
			{
				NsiInformationList: []models.NsiInformation{
					{
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
