// Copyright 2024 Ella Networks

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/smf/context"
)

func Start(dbInstance *db.Database) error {
	if dbInstance == nil {
		return fmt.Errorf("dbInstance is nil")
	}
	smfContext := context.SMF_Self()

	nodeId := context.NewNodeID("0.0.0.0")

	smfContext.CPNodeID = *nodeId

	smfContext.UserPlaneInformation = &context.UserPlaneInformation{
		UPNodes:              make(map[string]*context.UPNode),
		UPF:                  nil,
		AccessNetwork:        make(map[string]*context.UPNode),
		DefaultUserPlanePath: make(map[string][]*context.UPNode),
	}

	smfContext.DbInstance = dbInstance
	context.UpdateUserPlaneInformation()
	metrics.RegisterSmfMetrics()
	return nil
}
