package amf

import (
	"time"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/service"
)

var AMF = &service.AMF{}

const (
	SBI_PORT   = 29518
	NGAPP_PORT = 38412
)

func Start(ausfURL string, nrfURL string, nssfURL string, pcfURL string, smfURL string, UdmsdmURL string, UdmUecmURL string, webuiURL string) error {
	configuration := factory.Configuration{
		AmfName:      "AMF",
		NgapIpList:   []string{"0.0.0.0"},
		NgapPort:     NGAPP_PORT,
		SctpGrpcPort: 9000,
		Sbi: &factory.Sbi{
			BindingIPv4:  "0.0.0.0",
			Port:         SBI_PORT,
			RegisterIPv4: "0.0.0.0",
		},
		NetworkFeatureSupport5GS: &factory.NetworkFeatureSupport5GS{
			Emc:     0,
			EmcN3:   0,
			Emf:     0,
			Enable:  true,
			ImsVoPS: 0,
			IwkN26:  0,
			Mcsi:    0,
			Mpsi:    0,
		},
		ServiceNameList: []string{
			"namf-comm",
			"namf-evts",
			"namf-mt",
			"namf-loc",
			"namf-oam",
		},
		SupportDnnList: []string{"internet"},
		AusfUri:        ausfURL,
		NrfUri:         nrfURL,
		NssfUri:        nssfURL,
		PcfUri:         pcfURL,
		SmfUri:         smfURL,
		UdmsdmUri:      UdmsdmURL,
		UdmUecmUri:     UdmUecmURL,
		WebuiUri:       webuiURL,
		Security: &factory.Security{
			IntegrityOrder: []string{"NIA1", "NIA2"},
			CipheringOrder: []string{"NEA0"},
		},
		NetworkName: factory.NetworkName{
			Full:  "SDCORE5G",
			Short: "SDCORE",
		},
		T3502Value: 720,
		T3512Value: 3600,
		T3513: factory.TimerValue{
			Enable:        true,
			ExpireTime:    time.Duration(6 * time.Second),
			MaxRetryTimes: 4,
		},
		T3522: factory.TimerValue{
			Enable:        true,
			ExpireTime:    time.Duration(6 * time.Second),
			MaxRetryTimes: 4,
		},
		T3550: factory.TimerValue{
			Enable:        true,
			ExpireTime:    time.Duration(6 * time.Second),
			MaxRetryTimes: 4,
		},
		T3560: factory.TimerValue{
			Enable:        true,
			ExpireTime:    time.Duration(6 * time.Second),
			MaxRetryTimes: 4,
		},
		T3565: factory.TimerValue{
			Enable:        true,
			ExpireTime:    time.Duration(6 * time.Second),
			MaxRetryTimes: 4,
		},
	}
	config := factory.Config{
		Configuration: &configuration,
		Logger: &logger.Logger{
			AMF: &logger.LogSetting{
				DebugLevel: "debug",
			},
		},
		Info: &factory.Info{
			Version: "v1.0.0",
		},
	}
	AMF.Initialize(config)
	go AMF.Start()
	return nil
}
