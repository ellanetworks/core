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
	smfNodeID := context.NewNodeID("0.0.0.0")
	smfContext.CPNodeID = *smfNodeID
	upfNodeID := context.NewNodeID(config.UpfNodeID)
	upf := context.NewUPF(upfNodeID, config.DNN)
	smfContext.UPF = upf
	metrics.RegisterSmfMetrics()
	return nil
}
