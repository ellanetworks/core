// Copyright 2024 Ella Networks

package smf

import (
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/smf/context"
)

func Start(dbInstance *db.Database) {
	context.InitializeSMF(dbInstance)
}
