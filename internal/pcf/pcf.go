// Copyright 2024 Ella Networks

package pcf

import (
	"math"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
)

func Start(dbInstance *db.Database) error {
	pcfCtx = &PCFContext{
		TimeFormat:             "2006-01-02 15:04:05",
		PcfSuppFeats:           make(map[models.ServiceName]openapi.SupportedFeature),
		SessionRuleIDGenerator: idgenerator.NewGenerator(1, math.MaxInt64),
		QoSDataIDGenerator:     idgenerator.NewGenerator(1, math.MaxInt64),
		DbInstance:             dbInstance,
	}
	return nil
}
