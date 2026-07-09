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

// handleGUTIReallocationComplete finalises a standalone GUTI reallocation: it stops
// T3450 and commits the new GUTI, freeing the old one (TS 24.301 §5.4.1.4).
func handleGUTIReallocationComplete(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	if ue.EMMState() != mme.EMMRegistered {
		logger.From(ctx, logger.MmeLog).Warn("ignoring GUTI Reallocation Complete outside EMM-REGISTERED")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	if _, err := eps.ParseGUTIReallocationComplete(plain); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode GUTI Reallocation Complete", zap.Error(err))
		return nasreply.Handled()
	}

	ue.Conn().StopNASGuard()
	m.CommitGUTIRealloc(ue)

	return nasreply.Handled()
}
