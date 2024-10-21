package smf

import (
	"fmt"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/db/sql"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/service"
)

var SMF = &service.SMF{}

const SBI_PORT = 29502

func Start(amfURL string, pcfURL string, udmURL string, upfPfcpAddress string, dbQueries *sql.Queries) error {
	configuration := factory.Configuration{
		PFCP: &factory.PFCP{
			Addr: "0.0.0.0",
		},
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		AmfUri:         amfURL,
		PcfUri:         pcfURL,
		UdmUri:         udmURL,
		UpfPfcpAddress: upfPfcpAddress,
		SmfName:        "SMF",
		ServiceNameList: []string{
			"nsmf-pdusession",
			"nsmf-event-exposure",
			"nsmf-oam",
		},
		DBQueries: dbQueries,
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
