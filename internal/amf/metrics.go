// Copyright 2026 Ella Networks

package amf

import (
	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm"
	"github.com/ellanetworks/core/internal/amf/ngap"
)

func RegisterMetrics(amf *amfContext.AMF) {
	amfContext.RegisterMetrics(amf)
	gmm.RegisterMetrics()
	ngap.RegisterMetrics()
}
