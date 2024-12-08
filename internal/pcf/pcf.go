package pcf

import (
	"math"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/idgenerator"
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/internal/notifyevent"
	"go.uber.org/zap/zapcore"
)

func Start(amfURL string) error {
	level, err := zapcore.ParseLevel("debug")
	if err != nil {
		return err
	}
	logger.SetLogLevel(level)
	err = notifyevent.RegisterNotifyDispatcher()
	if err != nil {
		return err
	}
	context := context.PCF_Self()
	context.AmfUri = amfURL
	context.TimeFormat = "2006-01-02 15:04:05"
	context.PcfSuppFeats = make(map[models.ServiceName]openapi.SupportedFeature)
	context.SessionRuleIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	context.QoSDataIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	return nil
}
