package pcf

import (
	"math"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/internal/notifyevent"
	"github.com/yeastengine/ella/internal/util/idgenerator"
)

func Start(dbInstance *db.Database) error {
	err := notifyevent.RegisterNotifyDispatcher()
	if err != nil {
		return err
	}
	context := context.PCF_Self()
	context.TimeFormat = "2006-01-02 15:04:05"
	context.PcfSuppFeats = make(map[models.ServiceName]openapi.SupportedFeature)
	context.SessionRuleIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	context.QoSDataIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	context.DbInstance = dbInstance
	return nil
}
