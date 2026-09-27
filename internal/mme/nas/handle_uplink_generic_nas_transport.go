// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleUplinkGenericNASTransport(ctx context.Context, m *mme.MME, ue *mme.UeContext, msg *eps.UplinkGenericNASTransport) nasreply.Disposition {
	if ue.EMMState() != mme.EMMRegistered {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Uplink Generic NAS Transport outside EMM-REGISTERED")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	if msg.ContainerType != eps.GenericMessageContainerTypeLPP {
		logger.From(ctx, logger.MmeLog).Warn("unsupported generic message container type in Uplink Generic NAS Transport",
			zap.Stringer("container_type", msg.ContainerType))

		return nasreply.Handled()
	}

	if m.LPPHandler == nil {
		logger.From(ctx, logger.MmeLog).Error("LPP handler not configured")
		return nasreply.Handled()
	}

	if err := m.LPPHandler.ForwardLPP(ctx, ue.Supi(), msg.AdditionalInformation, msg.Container); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to forward LPP to LMF", zap.Error(err))
	}

	return nasreply.Handled()
}
