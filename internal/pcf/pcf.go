// Copyright 2024 Ella Networks

package pcf

import (
	"math"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/util/idgenerator"
)

func Start(dbInstance *db.Database) error {
	pcfCtx = &PCFContext{
		SessionRuleIDGenerator: idgenerator.NewGenerator(1, math.MaxInt64),
		QoSDataIDGenerator:     idgenerator.NewGenerator(1, math.MaxInt64),
		DBInstance:             dbInstance,
	}
	return nil
}
