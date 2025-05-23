// Copyright 2024 Ella Networks

package smf

import (
	ctx "context"
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
	smfContext.DBInstance = dbInstance
	nodeID := context.NewNodeID("0.0.0.0")
	smfContext.CPNodeID = *nodeID
	upNode, err := context.BuildUserPlaneInformationFromConfig(ctx.Background())
	if err != nil {
		return fmt.Errorf("failed to build user plane information from config: %v", err)
	}
	smfContext.UPF = upNode
	metrics.RegisterSmfMetrics()
	return nil
}
