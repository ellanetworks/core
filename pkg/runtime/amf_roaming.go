// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/sepp"
)

func wireAMFRoaming(amfInstance *amf.AMF, m *sepp.Manager) *roaming.AMFCallbacks {
	amfInstance.HomeNetwork = roaming.NewAMFHome(roaming.NewHomeSBI(m, roaming.NewDiscovery(m)))
	amfInstance.N32 = m

	return roaming.NewAMFCallbacks(amfInstance, logger.AmfLog)
}

type amfSessionVisitors struct {
	amf *amf.AMF
}

func (a amfSessionVisitors) SessionVisitor(imsi string) (roaming.SessionVisitor, bool) {
	v, ok := a.amf.VisitorHome(imsi)
	if !ok || !v.Registered || v.Subscription == nil {
		return roaming.SessionVisitor{}, false
	}

	return roaming.SessionVisitor{
		PartnerID:        v.PartnerID,
		HomePLMN:         v.Home.PLMN,
		ServingPLMN:      v.Home.Serving,
		RoutingIndicator: v.Home.RoutingIndicator,
		SMFSelection:     v.SmfSelection(),
	}, true
}
