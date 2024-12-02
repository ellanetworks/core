package smf

import (
	"fmt"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/service"
)

var SMF = &service.SMF{}

const SBI_PORT = 29502

func Start(amfURL string, pcfURL string, udmURL string) error {
	configuration := factory.Configuration{
		Logger: &logger.Logger{
			SMF: &logger.LogSetting{
				DebugLevel: "debug",
			},
		},
		PFCP: &factory.PFCP{
			Addr: "0.0.0.0",
		},
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		AmfUri:  amfURL,
		PcfUri:  pcfURL,
		UdmUri:  udmURL,
		SmfName: "SMF",
		ServiceNameList: []string{
			"nsmf-pdusession",
			"nsmf-event-exposure",
			"nsmf-oam",
		},
	}

	err := SMF.Initialize(configuration)
	if err != nil {
		return fmt.Errorf("failed to initialize SMF")
	}
	go SMF.Start()
	return nil
}
