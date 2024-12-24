package pcf

import (
	"math"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pcf/context"
	"github.com/ellanetworks/core/internal/util/idgenerator"
)

func Start(dbInstance *db.Database) error {
	context := context.PCF_Self()
	context.TimeFormat = "2006-01-02 15:04:05"
	context.PcfSuppFeats = make(map[models.ServiceName]openapi.SupportedFeature)
	context.SessionRuleIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	context.QoSDataIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	context.DbInstance = dbInstance
	return nil
}
