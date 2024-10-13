package pcf

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/service"
)

var PCF = &service.PCF{}

const SBI_PORT = 29507

func Start(amfURL string, nrfURL string, udrURL string, webuiURL string) error {
	configuration := factory.Configuration{
		PcfName: "PCF",
		Sbi: &factory.Sbi{
			BindingIPv4: "0.0.0.0",
			Port:        SBI_PORT,
		},
		DefaultBdtRefId: "BdtPolicyId-",
		AmfUri:          amfURL,
		NrfUri:          nrfURL,
		UdrUri:          udrURL,
		WebuiUri:        webuiURL,
		ServiceList: []factory.Service{
			{
				ServiceName: "npcf-am-policy-control",
			},
			{
				ServiceName: "npcf-smpolicycontrol",
				SuppFeat:    "3fff",
			},
			{
				ServiceName: "npcf-bdtpolicycontrol",
			},
			{
				ServiceName: "npcf-policyauthorization",
				SuppFeat:    "3",
			},
			{
				ServiceName: "npcf-eventexposure",
			},
			{
				ServiceName: "npcf-ue-policy-control",
			},
		},
	}
	config := factory.Config{
		Info: &factory.Info{
			Description: "PCF initial local configuration",
			Version:     "1.0.0",
		},
		Logger: &logger.Logger{
			PCF: &logger.LogSetting{
				DebugLevel:   "debug",
				ReportCaller: false,
			},
		},
		Configuration: &configuration,
	}
	PCF.Initialize(config)
	go PCF.Start()
	return nil
}
