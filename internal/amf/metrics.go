// Copyright 2026 Ella Networks

package amf

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm"
	"github.com/ellanetworks/core/internal/amf/ngap"
)

func RegisterMetrics() {
	context.RegisterMetrics()
	gmm.RegisterMetrics()
	ngap.RegisterMetrics()
}
