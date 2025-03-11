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
	smfContext.DBInstance = dbInstance
	upNode, err := context.BuildUserPlaneInformationFromConfig()
	if err != nil {
		return fmt.Errorf("failed to build user plane information from config: %v", err)
	}
	smfContext.UPF = upNode
	metrics.RegisterSmfMetrics()
	return nil
}
