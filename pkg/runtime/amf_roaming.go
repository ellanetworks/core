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
	amfInstance.HPLMN = roaming.NewAMFHPLMN(roaming.NewHPLMNSBI(m, roaming.NewDiscovery(m)))

	return roaming.NewAMFCallbacks(amfInstance, logger.AmfLog)
}
