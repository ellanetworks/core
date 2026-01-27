// Copyright 2026 Ella Networks

package smf

import (
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pdusession"
)

func RegisterMetrics() {
	context.RegisterMetrics()
	pdusession.RegisterMetrics()
}
