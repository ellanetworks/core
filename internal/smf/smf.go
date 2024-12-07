package smf

import (
	"fmt"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/service"
)

var SMF = &service.SMF{}

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
		AmfUri:  amfURL,
		PcfUri:  pcfURL,
		UdmUri:  udmURL,
		SmfName: "SMF",
	}

	err := SMF.Initialize(configuration)
	if err != nil {
		return fmt.Errorf("failed to initialize SMF")
	}
	go SMF.Start()
	return nil
}
