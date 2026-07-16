// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleULGenericNASTransport decodes an UPLINK GENERIC NAS TRANSPORT
// (TS 24.301 §8.2.31) and forwards its LPP container to the LMF. The Additional
// information IE carries the LCS correlation identifier the LMF routes the reply
// by.
func handleULGenericNASTransport(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	msg, err := eps.ParseULGenericNASTransport(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode UL Generic NAS Transport", zap.Error(err))
		return nasreply.StatusMM(nasreply.CauseInvalidMandatoryInfo)
	}

	if msg.ContainerType != eps.GenericContainerTypeLPP {
		logger.From(ctx, logger.MmeLog).Warn("UL Generic NAS Transport carries an unsupported container type",
			zap.Uint8("container_type", msg.ContainerType))

		return nasreply.Handled()
	}

	logger.From(ctx, logger.MmeLog).Info("UL Generic NAS Transport carries LPP payload",
		zap.String("supi", ue.Supi().String()),
		zap.Int("length", len(msg.Container)),
		zap.String("lpp_hex", hex.EncodeToString(msg.Container)),
		zap.String("additional_information", hex.EncodeToString(msg.AdditionalInfo)),
	)

	if m.LPPHandler == nil {
		logger.From(ctx, logger.MmeLog).Error("LPP handler not configured")
		return nasreply.Handled()
	}

	if err := m.LPPHandler.ForwardLPP(ctx, ue.Supi(), msg.AdditionalInfo, msg.Container); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to forward LPP to LMF", zap.Error(err))
	}

	return nasreply.Handled()
}
