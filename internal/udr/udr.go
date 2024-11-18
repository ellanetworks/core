package udr

import (
	"log"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/service"
	"github.com/yeastengine/ella/internal/udr/sql"
)

var UDR = &service.UDR{}

const SBI_PORT = 29504

func Start(mongoDBURL string, mongoDBName, webuiURL string, sqlDBPath string) error {
	dbQueries, err := sql.Initialize(sqlDBPath)
	if err != nil {
		log.Fatalf("failed to initialize sql database at %s: %v", sqlDBPath, err)
	}
	configuration := factory.Configuration{
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		Mongodb: &factory.Mongodb{
			Name:           mongoDBName,
			Url:            mongoDBURL,
			AuthKeysDbName: mongoDBName,
			AuthUrl:        mongoDBURL,
		},
		Sql: &factory.Sql{
			Path:    sqlDBPath,
			Queries: dbQueries,
		},
		WebuiUri: webuiURL,
	}
	config := factory.Config{
		Info: &factory.Info{
			Description: "UDR initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			UDR: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		Configuration: &configuration,
	}

	UDR.Initialize(config)
	go UDR.Start()
	return nil
}
