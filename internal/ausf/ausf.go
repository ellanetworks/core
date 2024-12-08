package ausf

import (
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/ausf/context"
	"github.com/yeastengine/ella/internal/ausf/factory"
	"go.uber.org/zap/zapcore"
)

const AUSF_GROUP_ID = "ausfGroup001"

func Start() error {
	configuration := factory.Configuration{
		GroupId: AUSF_GROUP_ID,
	}

	factory.InitConfigFactory(configuration)
	level, err := zapcore.ParseLevel("debug")
	if err != nil {
		return err
	}
	logger.SetLogLevel(level)
	context.Init()
	return nil
}
