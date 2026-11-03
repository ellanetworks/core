// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/prometheus/client_golang/prometheus"
)

type roamerSource interface {
	RoamerLoad() map[string]roaming.PartnerLoad
	Roamers() []roaming.LiveRoamer
}

func registerRoamers(reg prometheus.Registerer, ran5G, ran4G roamerSource) {
	roaming.RegisterMetrics(reg, ran5G.RoamerLoad, ran4G.RoamerLoad)

	server.SetLocalRoamers(func() []roaming.LiveRoamer {
		return append(ran5G.Roamers(), ran4G.Roamers()...)
	})
}
