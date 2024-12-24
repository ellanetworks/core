package amf

import (
	"time"

	"github.com/ellanetworks/core/internal/amf/factory"
	"github.com/ellanetworks/core/internal/amf/service"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
)

var AMF = &service.AMF{}

const (
	NGAPP_PORT = 38412
)

func Start(dbInstance *db.Database) error {
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
		SupportDnnList: []string{config.DNN},
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
			ExpireTime:    6 * time.Second,
			MaxRetryTimes: 4,
		},
		T3522: factory.TimerValue{
			Enable:        true,
			ExpireTime:    6 * time.Second,
			MaxRetryTimes: 4,
		},
		T3550: factory.TimerValue{
			Enable:        true,
			ExpireTime:    6 * time.Second,
			MaxRetryTimes: 4,
		},
		T3560: factory.TimerValue{
			Enable:        true,
			ExpireTime:    6 * time.Second,
			MaxRetryTimes: 4,
		},
		T3565: factory.TimerValue{
			Enable:        true,
			ExpireTime:    6 * time.Second,
			MaxRetryTimes: 4,
		},
		DBInstance: dbInstance,
	}

	factory.InitConfigFactory(configuration)
	AMF.Start()
	return nil
}
