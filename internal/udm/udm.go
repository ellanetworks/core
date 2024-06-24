package udm

import (
	"fmt"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udm/factory"
	"github.com/yeastengine/ella/internal/udm/service"
)

var UDM = &service.UDM{}

const (
	UDM_HNP_PRIVATE_KEY = "c09c17bddf23357f614f492075b970d825767718114f59554ce2f345cf8c4b6a"
	SBI_PORT            = 29503
)

func Start(nrfURL string, webuiURL string) error {
	configuration := factory.Configuration{
		UdmName: "UDM",
		Sbi: &factory.Sbi{
			RegisterIPv4: "0.0.0.0",
			BindingIPv4:  "0.0.0.0",
			Port:         SBI_PORT,
		},
		ServiceNameList: []string{
			"nudm-sdm",
			"nudm-uecm",
			"nudm-ueau",
			"nudm-ee",
			"nudm-pp",
		},
		NrfUri:   nrfURL,
		WebuiUri: webuiURL,
		Keys: &factory.Keys{
			UdmProfileAHNPrivateKey: UDM_HNP_PRIVATE_KEY,
		},
	}
	config := factory.Config{
		Info: &factory.Info{
			Description: "UDM initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			UDM: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		Configuration: &configuration,
	}
	err := UDM.Initialize(config)
	if err != nil {
		return fmt.Errorf("failed to initialize")
	}
	go UDM.Start()
	return nil
}
