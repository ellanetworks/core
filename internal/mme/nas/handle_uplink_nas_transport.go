// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

func handleUplinkNASTransport(ctx context.Context, m *mme.MME, ue *mme.UeContext, msg *eps.UplinkNASTransport) nasreply.Disposition {
	if ue.EMMState() != mme.EMMRegistered {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Uplink NAS Transport outside EMM-REGISTERED")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	m.ForwardSMS(ctx, ue, msg.NASMessageContainer)

	return nasreply.Handled()
}
