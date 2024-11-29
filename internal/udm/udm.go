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

func Start(udrURL string) error {
	configuration := factory.Configuration{
		Logger: &logger.Logger{
			UDM: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		UdmName: "UDM",
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		ServiceNameList: []string{
			"nudm-sdm",
			"nudm-uecm",
			"nudm-ueau",
			"nudm-ee",
			"nudm-pp",
		},
		UdrUri: udrURL,
		Keys: &factory.Keys{
			UdmProfileAHNPrivateKey: UDM_HNP_PRIVATE_KEY,
		},
	}
	err := UDM.Initialize(configuration)
	if err != nil {
		return fmt.Errorf("failed to initialize")
	}
	go UDM.Start()
	return nil
}
