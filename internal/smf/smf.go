// Copyright 2024 Ella Networks

package smf

import (
	"fmt"
	"net"

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
	smfContext.CPNodeID = net.ParseIP("0.0.0.0")
	upfNodeID := net.ParseIP(config.UpfNodeID)
	smfContext.UPF = context.NewUPF(upfNodeID)
	metrics.RegisterSmfMetrics()
	return nil
}
