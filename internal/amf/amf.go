package amf

import (
	"time"

	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/service"
	"go.uber.org/zap/zapcore"
)

var AMF = &service.AMF{}

const (
	NGAPP_PORT = 38412
)

func Start() error {
	configuration := factory.Configuration{
		AmfName:      "AMF",
		NgapIpList:   []string{"0.0.0.0"},
		NgapPort:     NGAPP_PORT,
		SctpGrpcPort: 9000,
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
		SupportDnnList: []string{"internet"},
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

	factory.InitConfigFactory(configuration)
	level, err := zapcore.ParseLevel("debug")
	if err != nil {
		return err
	}
	logger.SetLogLevel(level)
	AMF.Start()
	return nil
}
