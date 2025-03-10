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
	smfContext := context.SMFSelf()

	nodeID := context.NewNodeID("0.0.0.0")

	smfContext.CPNodeID = *nodeID

	smfContext.UserPlaneInformation = &context.UserPlaneInformation{
		UPNodes:       make(map[string]*context.UPNode),
		UPF:           nil,
		AccessNetwork: make(map[string]*context.UPNode),
	}

	smfContext.DBInstance = dbInstance
	context.UpdateUserPlaneInformation()
	metrics.RegisterSmfMetrics()
	return nil
}
