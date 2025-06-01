// Copyright 2024 Ella Networks

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/smf/context"
)

func Start(dbInstance *db.Database) error {
	if dbInstance == nil {
		return fmt.Errorf("dbInstance is nil")
	}
	smfContext := context.SMFSelf()
	smfContext.DBInstance = dbInstance
	smfContext.CPNodeID = *context.NewNodeID("0.0.0.0")
	upfNodeID := context.NewNodeID(config.UpfNodeID)
	smfContext.UPF = context.NewUPF(upfNodeID, config.DNN)
	metrics.RegisterSmfMetrics()
	return nil
}
